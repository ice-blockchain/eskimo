// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	emaillink "github.com/ice-blockchain/eskimo/auth/email_link"
	"github.com/ice-blockchain/wintr/auth"
	"github.com/ice-blockchain/wintr/server"
	"github.com/ice-blockchain/wintr/terror"
)

func (s *service) setupAuthRoutes(router *server.Router) {
	router.
		Group("v1w").
		POST("auth/sendSignInLinkToEmail", server.RootHandler(s.SendSignInLinkToEmail)).
		POST("auth/refreshTokens", server.RootHandler(s.RegenerateTokens)).
		POST("auth/signInWithEmailLink", server.RootHandler(s.SignIn)).
		POST("auth/getConfirmationStatus", server.RootHandler(s.Status))
		GET("auth/account", server.RootHandler(s.Account))
}

// SendSignInLinkToEmail godoc
//
//	@Schemes
//	@Description	Starts email link auth process
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		SendSignInLinkToEmailRequestArg	true	"Request params"
//	@Success		200		{object}	Auth
//	@Failure		409		{object}	server.ErrorResponse	"if email conflicts with another user's"
//	@Failure		422		{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500		{object}	server.ErrorResponse
//	@Failure		504		{object}	server.ErrorResponse	"if request times out"
//	@Router			/auth/sendSignInLinkToEmail [POST].
func (s *service) SendSignInLinkToEmail( //nolint:gocritic // .
	ctx context.Context,
	req *server.Request[SendSignInLinkToEmailRequestArg, Auth],
) (*server.Response[Auth], *server.Response[server.ErrorResponse]) {
	loginSession, err := s.authEmailLinkClient.SendSignInLinkToEmail(ctx, req.Data.Email, req.Data.DeviceUniqueID, req.Data.Language)
	if err != nil {
		switch {
		case errors.Is(err, emaillink.ErrUserBlocked):
			return nil, server.BadRequest(err, userBlockedErrorCode)
		case errors.Is(err, emaillink.ErrUserDuplicate):
			return nil, server.Conflict(err, duplicateUserErrorCode)
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
func (s *service) SignIn( //nolint:gocritic // .
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
	tokens, err := s.authEmailLinkClient.RegenerateTokens(ctx, tokenPayload, req.Data.CustomClaims)
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

// Account godoc
//
//	@Schemes
//	@Description	Fetches user's account based on token's data
//	@Tags			Auth
//	@Produce		json
//	@Param			Authorization	header		string	true	"Insert your access token"	default(Bearer <Add access token here>)
//	@Success		200				{object}	User
//	@Failure		404				{object}	server.ErrorResponse	"if user do not have an account yet"
//	@Failure		500				{object}	server.ErrorResponse
//	@Failure		504				{object}	server.ErrorResponse	"if request times out"
//	@Router			/auth/account [GET].
func (s *service) Account( //nolint:funlen,gocognit,gocritic,revive // Fallback logic with iceID
	ctx context.Context,
	req *server.Request[GetAccountArg, Profile],
) (*server.Response[Profile], *server.Response[server.ErrorResponse]) {
	usr, err := s.usersProcessor.GetUserByID(ctx, req.AuthenticatedUser.UserID)
	if err != nil { //nolint:nestif // Fallback logic.
		if errors.Is(err, users.ErrNotFound) {
			iceID, iErr := s.authEmailLinkClient.IceUserID(ctx, req.AuthenticatedUser.Email)
			if iErr != nil {
				return nil, server.NotFound(multierror.Append(
					errors.Wrapf(err, "user with id `%v` was not found", req.AuthenticatedUser.UserID),
					errors.Wrapf(iErr, "failed to fetch iceID for email `%v`", req.AuthenticatedUser.Email),
				).ErrorOrNil(), userNotFoundErrorCode)
			}
			if iceID != "" {
				iceIDCtx := context.WithValue(ctx, users.RequestingUserIDCtxValueKey, iceID) //nolint:staticcheck // To bypass profile ownership check.
				usr, iErr = s.usersProcessor.GetUserByID(iceIDCtx, iceID)
				if iErr != nil {
					if errors.Is(iErr, users.ErrNotFound) {
						return nil, server.NotFound(multierror.Append(
							errors.Wrapf(err, "user with id '%v'(fb) was not found", req.AuthenticatedUser.UserID),
							errors.Wrapf(iErr, "user with id '%v'(ice) was not found", iceID),
						).ErrorOrNil(), userNotFoundErrorCode)
					}

					return nil, server.Unexpected(err)
				}
				if req.AuthenticatedUser.IsFirebase() {
					md := users.JSON(map[string]any{
						auth.FirebaseIDClaim: req.AuthenticatedUser.UserID,
					})
					if _, err = s.authEmailLinkClient.UpdateMetadata(ctx, usr.ID, &md); err != nil {
						return nil, server.Unexpected(err)
					}
				}

				return server.OK(&Profile{UserProfile: usr, Checksum: usr.Checksum()}), nil
			}

			return nil, server.NotFound(errors.Wrapf(err, "user with id `%v` was not found", req.AuthenticatedUser.UserID), userNotFoundErrorCode)
		}

		return nil, server.Unexpected(errors.Wrapf(err, "failed to get user by id: %v", req.AuthenticatedUser.UserID))
	}

	return server.OK(&Profile{UserProfile: usr, Checksum: usr.Checksum()}), nil
}
