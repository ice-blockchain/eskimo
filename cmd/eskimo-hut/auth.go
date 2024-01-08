// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"
	"net/mail"
	"strings"
	stdlibtime "time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	emaillink "github.com/ice-blockchain/eskimo/auth/email_link"
	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/auth"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/server"
	"github.com/ice-blockchain/wintr/terror"
	"github.com/ice-blockchain/wintr/time"
)

func (s *service) setupAuthRoutes(router *server.Router) {
	router.
		Group("v1w").
		POST("auth/sendSignInLinkToEmail", server.RootHandler(s.SendSignInLinkToEmail)).
		POST("auth/refreshTokens", server.RootHandler(s.RegenerateTokens)).
		POST("auth/signInWithEmailLink", server.RootHandler(s.SignIn)).
		POST("auth/getConfirmationStatus", server.RootHandler(s.Status)).
		POST("auth/getMetadata", server.RootHandler(s.Metadata)).
		POST("auth/processFaceRecognitionResult", server.RootHandler(s.ProcessFaceRecognitionResult)).
		POST("auth/getValidUserForPhoneNumberMigration", server.RootHandler(s.GetValidUserForPhoneNumberMigration))
}

// SendSignInLinkToEmail godoc
//
//	@Schemes
//	@Description	Starts email link auth process
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			request			body		SendSignInLinkToEmailRequestArg	true	"Request params"
//	@Param			X-API-Key		header		string							false	"Insert your api key"							default(<Add api key here>)
//	@Param			X-User-ID		header		string							false	"UserID to process phone number migration for"	default()
//	@Param			X-Forwarded-For	header		string							false	"Client IP"										default(1.1.1.1)
//	@Success		200				{object}	Auth
//	@Failure		403				{object}	server.ErrorResponse	"if too many pending auth requests from one IP"
//	@Failure		409				{object}	server.ErrorResponse	"if email conflicts with another user's"
//	@Failure		422				{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500				{object}	server.ErrorResponse
//	@Failure		504				{object}	server.ErrorResponse	"if request times out"
//	@Router			/auth/sendSignInLinkToEmail [POST].
func (s *service) SendSignInLinkToEmail( //nolint:gocritic,funlen // .
	ctx context.Context,
	req *server.Request[SendSignInLinkToEmailRequestArg, Auth],
) (*server.Response[Auth], *server.Response[server.ErrorResponse]) {
	if req.Data.UserID != "" && cfg.APIKey != req.Data.APIKey {
		return nil, server.Forbidden(errors.New("not allowed"))
	}
	email := strings.TrimSpace(strings.ToLower(req.Data.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, server.BadRequest(err, invalidEmail)
	}
	ctx = emaillink.ContextWithPhoneNumberToEmailMigration(ctx, req.Data.UserID) //nolint:revive // Not a problem.
	loginSession, err := s.authEmailLinkClient.SendSignInLinkToEmail(ctx, email, req.Data.DeviceUniqueID, req.Data.Language, req.ClientIP.String())
	if err != nil {
		switch {
		case errors.Is(err, emaillink.ErrUserBlocked):
			if tErr := terror.As(err); tErr != nil {
				return nil, server.BadRequest(err, userBlockedErrorCode, tErr.Data)
			}
		case errors.Is(err, emaillink.ErrUserDuplicate):
			if tErr := terror.As(err); tErr != nil {
				return nil, server.Conflict(err, duplicateUserErrorCode, tErr.Data)
			}
		case errors.Is(err, emaillink.ErrTooManyAttempts):
			if tErr := terror.As(err); tErr != nil {
				return nil, server.ForbiddenWithCode(err, tooManyRequests, tErr.Data)
			}
		default:
			return nil, server.Unexpected(errors.Wrapf(err, "failed to start email link auth %#v", req.Data))
		}
	}

	return server.OK[Auth](&Auth{LoginSession: loginSession}), nil
}

// SignIn godoc
//
//	@Schemes
//	@Description	Finishes login flow using magic link
//	@Tags			Auth
//	@Produce		json
//	@Param			request	body		MagicLinkPayload	true	"Request params"
//	@Success		200		{object}	any
//	@Failure		400		{object}	server.ErrorResponse	"if invalid or expired payload provided"
//	@Failure		404		{object}	server.ErrorResponse	"if email does not need to be confirmed by magic link"
//	@Failure		422		{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500		{object}	server.ErrorResponse
//	@Failure		504		{object}	server.ErrorResponse	"if request times out"
//	@Router			/auth/signInWithEmailLink [POST].
func (s *service) SignIn( //nolint:gocritic //.
	ctx context.Context,
	req *server.Request[MagicLinkPayload, any],
) (*server.Response[any], *server.Response[server.ErrorResponse]) {
	if err := s.authEmailLinkClient.SignIn(ctx, req.Data.EmailToken, req.Data.ConfirmationCode); err != nil {
		err = errors.Wrapf(err, "finish login using magic link failed for %#v", req.Data)
		switch {
		case errors.Is(err, users.ErrRaceCondition):
			return nil, server.BadRequest(err, raceConditionErrorCode)
		case errors.Is(err, users.ErrNotFound):
			return nil, server.NotFound(err, userNotFoundErrorCode)
		case errors.Is(err, users.ErrDuplicate):
			if tErr := terror.As(err); tErr != nil {
				return nil, server.Conflict(err, duplicateUserErrorCode, tErr.Data)
			}
		case errors.Is(err, emaillink.ErrNoConfirmationRequired):
			return nil, server.NotFound(err, confirmationCodeNotFoundErrorCode)
		case errors.Is(err, emaillink.ErrExpiredToken):
			return nil, server.BadRequest(err, linkExpiredErrorCode)
		case errors.Is(err, emaillink.ErrInvalidToken):
			return nil, server.BadRequest(err, invalidOTPCodeErrorCode)
		case errors.Is(err, emaillink.ErrConfirmationCodeAttemptsExceeded):
			return nil, server.BadRequest(err, confirmationCodeAttemptsExceededErrorCode)
		case errors.Is(err, emaillink.ErrConfirmationCodeWrong):
			return nil, server.BadRequest(err, confirmationCodeWrongErrorCode)
		default:
			return nil, server.Unexpected(err)
		}
	}

	return server.OK[any](), nil
}

// RegenerateTokens godoc
//
//	@Schemes
//	@Description	Issues new access token
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string			true	"Insert your access token"	default(Bearer <Add access token here>)
//	@Param			request			body		RefreshToken	true	"Body containing customClaims"
//	@Success		200				{object}	RefreshedToken
//	@Failure		400				{object}	server.ErrorResponse	"if users data from token does not match data in db"
//	@Failure		403				{object}	server.ErrorResponse	"if invalid or expired refresh token provided"
//	@Failure		404				{object}	server.ErrorResponse	"if user or confirmation not found"
//	@Failure		422				{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500				{object}	server.ErrorResponse
//	@Failure		504				{object}	server.ErrorResponse	"if request times out"
//	@Router			/auth/refreshTokens [POST].
func (s *service) RegenerateTokens( //nolint:gocritic // .
	ctx context.Context,
	req *server.Request[RefreshToken, RefreshedToken],
) (*server.Response[RefreshedToken], *server.Response[server.ErrorResponse]) {
	tokenPayload := strings.TrimPrefix(req.Data.Authorization, "Bearer ")
	tokens, err := s.authEmailLinkClient.RegenerateTokens(ctx, tokenPayload)
	if err != nil {
		switch {
		case errors.Is(err, emaillink.ErrUserNotFound):
			return nil, server.NotFound(err, userNotFoundErrorCode)
		case errors.Is(err, emaillink.ErrExpiredToken):
			return nil, server.Forbidden(err)
		case errors.Is(err, emaillink.ErrInvalidToken):
			return nil, server.Forbidden(err)
		case errors.Is(err, emaillink.ErrUserDataMismatch):
			return nil, server.BadRequest(err, dataMismatchErrorCode)
		default:
			return nil, server.Unexpected(err)
		}
	}

	return server.OK(&RefreshedToken{Tokens: tokens}), nil
}

// Status godoc
//
//	@Schemes
//	@Description	Status of the auth process
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		StatusArg	true	"Request params"
//	@Success		200		{object}	Auth
//	@Failure		422		{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		403		{object}	server.ErrorResponse	"if invalid or expired login session provided"
//	@Failure		404		{object}	server.ErrorResponse	"if login session not found or confirmation code verifying failed"
//	@Failure		500		{object}	server.ErrorResponse
//	@Failure		504		{object}	server.ErrorResponse	"if request times out"
//	@Router			/auth/getConfirmationStatus [POST].
func (s *service) Status( //nolint:gocritic // .
	ctx context.Context,
	req *server.Request[StatusArg, Status],
) (*server.Response[Status], *server.Response[server.ErrorResponse]) {
	tokens, emailConfirmed, err := s.authEmailLinkClient.Status(ctx, req.Data.LoginSession)
	if err != nil {
		err = errors.Wrapf(err, "failed to get status for: %#v", req.Data)
		if err != nil {
			switch {
			case errors.Is(err, emaillink.ErrNoPendingLoginSession):
				return nil, server.NotFound(err, noPendingLoginSessionErrorCode)
			case errors.Is(err, emaillink.ErrStatusNotVerified):
				return server.OK(&Status{}), nil
			case errors.Is(err, emaillink.ErrInvalidToken):
				return nil, server.Forbidden(err)
			case errors.Is(err, emaillink.ErrExpiredToken):
				return nil, server.Forbidden(err)
			default:
				return nil, server.Unexpected(err)
			}
		}
	}
	if emailConfirmed {
		tokens = nil
	}

	return server.OK(&Status{
		RefreshedToken: &RefreshedToken{Tokens: tokens},
		EmailConfirmed: emailConfirmed,
	}), nil
}

// Metadata godoc
//
//	@Schemes
//	@Description	Fetches user's metadata based on token's data
//	@Tags			Auth
//	@Produce		json
//	@Param			Authorization	header		string	true	"Insert your access token"	default(Bearer <Add access token here>)
//	@Success		200				{object}	Metadata
//	@Failure		404				{object}	server.ErrorResponse	"if user do not have a metadata yet"
//	@Failure		500				{object}	server.ErrorResponse
//	@Failure		504				{object}	server.ErrorResponse	"if request times out"
//	@Router			/auth/getMetadata [POST].
func (s *service) Metadata(
	ctx context.Context,
	req *server.Request[GetMetadataArg, Metadata],
) (successResp *server.Response[Metadata], errorResp *server.Response[server.ErrorResponse]) {
	md, mdFields, err := s.authEmailLinkClient.Metadata(ctx, req.AuthenticatedUser.UserID, req.AuthenticatedUser.Email)
	if err != nil {
		switch {
		case errors.Is(err, emaillink.ErrUserNotFound):
			return s.findMetadataUsingIceID(ctx, &req.AuthenticatedUser, err)
		case errors.Is(err, emaillink.ErrUserDataMismatch):
			if fbErr := s.handleFirebaseEmailMismatch(ctx, &req.AuthenticatedUser, err); fbErr != nil {
				return nil, server.BadRequest(fbErr, dataMismatchErrorCode)
			}
		default:
			return nil, server.Unexpected(errors.Wrapf(err, "failed to get metadata for user by id: %v", req.AuthenticatedUser.UserID))
		}
	}
	if req.AuthenticatedUser.IsFirebase() {
		var updMD string
		if updMD, err = s.updateMetadataWithFirebaseID(ctx, &req.AuthenticatedUser, mdFields, req.AuthenticatedUser.UserID); err != nil {
			return nil, server.Unexpected(err)
		} else if updMD != "" {
			md = updMD
		}
	}

	return server.OK(&Metadata{Metadata: md, UserID: req.AuthenticatedUser.UserID}), nil
}

//nolint:funlen,gocognit,revive // .
func (s *service) findMetadataUsingIceID(ctx context.Context, loggedInUser *server.AuthenticatedUser, err error) (
	successResp *server.Response[Metadata],
	errorResp *server.Response[server.ErrorResponse],
) {
	var md string
	var mdFields *users.JSON
	iceID, iErr := s.authEmailLinkClient.IceUserID(ctx, strings.ToLower(loggedInUser.Email))
	if iErr != nil {
		return nil, server.NotFound(multierror.Append(
			errors.Wrapf(err, "metadata for user with id `%v` was not found", loggedInUser.UserID),
			errors.Wrapf(iErr, "failed to fetch iceID for email `%v`", loggedInUser.Email),
		).ErrorOrNil(), metadataNotFoundErrorCode)
	}
	if iceID != "" { //nolint:nestif // Error processing
		md, mdFields, iErr = s.authEmailLinkClient.Metadata(ctx, iceID, loggedInUser.Email)
		if iErr != nil {
			switch {
			case errors.Is(iErr, emaillink.ErrUserNotFound):
				return server.OK(&Metadata{UserID: iceID}), nil
			case errors.Is(iErr, emaillink.ErrUserDataMismatch):
				if fbErr := s.handleFirebaseEmailMismatch(ctx, loggedInUser, iErr); fbErr != nil {
					return nil, server.BadRequest(fbErr, dataMismatchErrorCode)
				}
			default:
				return nil, server.Unexpected(iErr)
			}
		}
		if loggedInUser.IsFirebase() {
			var mdUpd string
			if mdUpd, err = s.updateMetadataWithFirebaseID(ctx, loggedInUser, mdFields, iceID); err != nil {
				return nil, server.Unexpected(err)
			} else if mdUpd != "" {
				md = mdUpd
			}
		}

		return server.OK(&Metadata{Metadata: md, UserID: iceID}), nil
	}

	return nil, server.NotFound(errors.Wrapf(err, "metadata for user with id `%v` was not found", loggedInUser.UserID), metadataNotFoundErrorCode)
}

func (*service) handleFirebaseEmailMismatch(ctx context.Context, loggedInUser *server.AuthenticatedUser, err error) error {
	emailErr := terror.As(err)
	actualEmail := emailErr.Data["email"].(string) //nolint:forcetypeassert,revive,errcheck // .
	if !loggedInUser.IsFirebase() {
		return errors.Wrapf(emaillink.ErrUserDataMismatch, "actual email is %v, requested for %v", actualEmail, loggedInUser.Email)
	}

	fbClaimInterface, hasFBClaim := loggedInUser.Claims["firebase"]
	if hasFBClaim {
		signInWithInterface, hasSignInProvider := fbClaimInterface.(map[string]any)["sign_in_provider"]
		if hasSignInProvider {
			if signInProvider := signInWithInterface.(string); signInProvider != "password" { //nolint:forcetypeassert,revive,errcheck // .
				return nil
			}
		}
	}
	if fbErr := server.Auth(ctx).UpdateEmail(ctx, loggedInUser.UserID, actualEmail); fbErr != nil {
		if strings.Contains(fbErr.Error(), "conflicts with another user") {
			log.Warn("actual email is %v, requested for %v and failed to update in firebase %v", actualEmail, loggedInUser.Email, fbErr.Error())

			return nil
		}

		return errors.Wrapf(
			emaillink.ErrUserDataMismatch,
			"actual email is %v, requested for %v and failed to update in firebase %v",
			actualEmail, loggedInUser.Email, fbErr.Error(),
		)
	}

	return nil
}

func (s *service) updateMetadataWithFirebaseID(
	ctx context.Context,
	loggedInUser *server.AuthenticatedUser,
	mdFields *users.JSON,
	userID string,
) (md string, err error) {
	fields := mdFields //nolint:ifshort // .
	if fields == nil {
		empty := users.JSON(map[string]any{})
		fields = &empty
	}
	if _, hasFirebaseID := (*fields)[auth.FirebaseIDClaim]; !hasFirebaseID {
		var updatedMetadata *users.JSON
		mdToUpdate := users.JSON(map[string]any{
			auth.FirebaseIDClaim: loggedInUser.UserID,
		})
		if updatedMetadata, err = s.authEmailLinkClient.UpdateMetadata(ctx, userID, &mdToUpdate); err != nil {
			return "", errors.Wrapf(err, "can't update metadata for userIDID:%v", userID)
		}
		if updatedMetadata != nil {
			if md, err = server.Auth(ctx).GenerateMetadata(time.Now(), loggedInUser.UserID, *updatedMetadata); err != nil {
				return "", errors.Wrapf(err, "can't generate metadata for:%v", loggedInUser.UserID)
			}
		}
	}

	return md, nil
}

// ProcessFaceRecognitionResult godoc
//
//	@Schemes
//	@Description	Webhook to notify the service about the result of an user's face authentication process.
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string							true	"Insert your access token"	default(Bearer <Add access token here>)
//	@Param			X-API-Key		header	string							true	"Insert your api key"		default(<Add api key here>)
//	@Param			X-User-ID		header	string							false	"UserID to process"			default()
//	@Param			request			body	ProcessFaceRecognitionResultArg	true	"Request params"
//	@Success		200				"OK"
//	@Failure		401				{object}	server.ErrorResponse	"if not authenticated"
//	@Failure		403				{object}	server.ErrorResponse	"if not allowed"
//	@Failure		404				{object}	server.ErrorResponse	"if user not found"
//	@Failure		422				{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500				{object}	server.ErrorResponse
//	@Failure		504				{object}	server.ErrorResponse	"if request times out"
//	@Router			/auth/processFaceRecognitionResult [POST].
func (s *service) ProcessFaceRecognitionResult(
	ctx context.Context,
	req *server.Request[ProcessFaceRecognitionResultArg, any],
) (successResp *server.Response[any], errorResp *server.Response[server.ErrorResponse]) {
	if cfg.APIKey != req.Data.APIKey {
		return nil, server.Forbidden(errors.New("not allowed"))
	}
	usr, err := parseProcessFaceRecognitionResultRequest(req)
	if err != nil {
		if errors.Is(err, errNoPermission) {
			return nil, server.Forbidden(err)
		}

		return nil, server.UnprocessableEntity(err, invalidPropertiesErrorCode)
	}
	if err = s.usersProcessor.ModifyUser(ctx, usr, nil); err != nil {
		err = errors.Wrapf(err, "failed to UpdateFaceRecognitionResult for %#v", usr)
		switch {
		case errors.Is(err, users.ErrNotFound):
			return nil, server.NotFound(err, userNotFoundErrorCode)
		default:
			return nil, server.Unexpected(err)
		}
	}

	return server.OK[any](), nil
}

//nolint:funlen //.
func parseProcessFaceRecognitionResultRequest(req *server.Request[ProcessFaceRecognitionResultArg, any]) (*users.User, error) {
	lastUpdatedAtDates := make([]*time.Time, 0, len(req.Data.LastUpdatedAt))
	for ix, lastUpdatedAt := range req.Data.LastUpdatedAt {
		parsedLastUpdatedAt, err := stdlibtime.Parse(stdlibtime.RFC3339Nano, lastUpdatedAt)
		if err != nil {
			err = errors.Wrapf(err, "invalid `RFC3339` format for lastUpdatedAt[%v]=`%v`", ix, lastUpdatedAt)

			return nil, err
		}
		lastUpdatedAtDates = append(lastUpdatedAtDates, time.New(parsedLastUpdatedAt))
	}
	usr := new(users.User)
	usr.ID = req.AuthenticatedUser.UserID
	if req.Data.UserID != "" {
		if req.AuthenticatedUser.Role != adminRole {
			return nil, errors.Wrapf(errNoPermission, "insufficient role: %v, admin role required", req.AuthenticatedUser.Role)
		}
		usr.ID = req.Data.UserID
	}
	if len(lastUpdatedAtDates) > 0 {
		usr.KYCStepsLastUpdatedAt = &lastUpdatedAtDates
	} else {
		var nilDates []*time.Time
		usr.KYCStepsLastUpdatedAt = &nilDates
		usr.KYCStepsCreatedAt = &nilDates
	}
	kycStepPassed := users.KYCStep(len(lastUpdatedAtDates))
	usr.KYCStepPassed = &kycStepPassed
	switch {
	case *req.Data.Disabled:
		kycStepBlocked := users.FacialRecognitionKYCStep
		usr.KYCStepBlocked = &kycStepBlocked
	case req.Data.PotentiallyDuplicate != nil && *req.Data.PotentiallyDuplicate:
		kycStepBlocked := users.LivenessDetectionKYCStep
		usr.KYCStepBlocked = &kycStepBlocked
	default:
		kycStepBlocked := users.NoneKYCStep
		usr.KYCStepBlocked = &kycStepBlocked
	}

	return usr, nil
}

// GetValidUserForPhoneNumberMigration godoc
//
//	@Schemes
//	@Description	Returns minimal user information based on provided phone number, in the context of migrating a phone number only account to an email one.
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			phoneNumber	query		string	true	"the phone number to identify the account based on"
//	@Param			email		query		string	false	"the email to be linked to the account"
//	@Success		200			{object}	User
//	@Failure		400			{object}	server.ErrorResponse	"code:INVALID_EMAIL if email is invalid"
//	@Failure		403			{object}	server.ErrorResponse	"code:ACCOUNT_LOST if account lost"
//	@Failure		404			{object}	server.ErrorResponse	"code:USER_NOT_FOUND if user not found"
//	@Failure		409			{object}	server.ErrorResponse	"code:EMAIL_ALREADY_SET if email already set;code:EMAIL_USED_BY_SOMEBODY_ELSE if email use"
//	@Failure		422			{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500			{object}	server.ErrorResponse
//	@Failure		504			{object}	server.ErrorResponse	"if request times out"
//	@Router			/auth/getValidUserForPhoneNumberMigration [POST].
func (s *service) GetValidUserForPhoneNumberMigration( //nolint:funlen // .
	ctx context.Context,
	req *server.Request[GetValidUserForPhoneNumberMigrationArg, User],
) (successResp *server.Response[User], errorResp *server.Response[server.ErrorResponse]) {
	req.Data.Email = strings.TrimSpace(strings.ToLower(req.Data.Email))
	if _, err := mail.ParseAddress(req.Data.Email); req.Data.Email != "" && err != nil {
		return nil, server.BadRequest(err, invalidEmail)
	}

	usr, err := s.usersProcessor.GetUserByPhoneNumber(ctx, req.Data.PhoneNumber)
	if err != nil {
		return nil, server.Unexpected(errors.Wrapf(err, "failed to GetUserByPhoneNumber(%v)", req.Data.PhoneNumber))
	}

	switch {
	case usr == nil:
		return nil, server.NotFound(users.ErrNotFound, userNotFoundErrorCode)
	case usr.Email != "":
		return nil, server.Conflict(users.ErrDuplicate, emailAlreadySetErrorCode)
	case !usr.IsHuman():
		return nil, server.ForbiddenWithCode(errors.New("account is lost"), accountLostErrorCode)
	}

	emailUsedBySomebodyElse, err := s.usersProcessor.IsEmailUsedBySomebodyElse(ctx, usr.ID, req.Data.Email)
	if err != nil {
		if errors.Is(err, users.ErrDuplicate) {
			return nil, server.Conflict(users.ErrDuplicate, emailAlreadySetErrorCode)
		}

		return nil, server.Unexpected(errors.Wrapf(err, "failed to IsEmailUsedBySomebodyElse(%v,%v)", usr.ID, req.Data.Email))
	} else if emailUsedBySomebodyElse {
		return nil, server.Conflict(users.ErrDuplicate, emailUsedBySomebodyElseEmail)
	}

	minimalUsr := new(User)
	minimalUsr.User = new(users.User)
	minimalUsr.ID = usr.ID

	return server.OK(minimalUsr), nil
}
