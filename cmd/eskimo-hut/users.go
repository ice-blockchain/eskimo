// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"

	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	emaillink "github.com/ice-blockchain/eskimo/auth/email_link"
	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/auth"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/server"
	"github.com/ice-blockchain/wintr/terror"
)

func (s *service) setupUserRoutes(router *server.Router) {
	router.
		Group("v1w").
		POST("users", server.RootHandler(s.CreateUser)).
		PATCH("users/:userId", server.RootHandler(s.ModifyUser)).
		DELETE("users/:userId", server.RootHandler(s.DeleteUser))
}

// CreateUser godoc
//
//	@Schemes
//	@Description	Creates an user account
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			Authorization		header		string					true	"Insert your access token"		default(Bearer <Add access token here>)
//	@Param			X-Forwarded-For		header		string					false	"Client IP"						default(1.1.1.1)
//	@Param			X-Account-Metadata	header		string					false	"Insert your metadata token"	default(<Add metadata token here>)
//	@Param			request				body		CreateUserRequestBody	true	"Request params"
//	@Success		201					{object}	User
//	@Failure		400					{object}	server.ErrorResponse	"if validations fail"
//	@Failure		401					{object}	server.ErrorResponse	"if not authorized"
//	@Failure		409					{object}	server.ErrorResponse	"user already exists with that ID, email or phone number"
//	@Failure		422					{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500					{object}	server.ErrorResponse
//	@Failure		504					{object}	server.ErrorResponse	"if request times out"
//	@Router			/users [POST].
func (s *service) CreateUser( //nolint:funlen,gocritic // .
	ctx context.Context,
	req *server.Request[CreateUserRequestBody, User],
) (*server.Response[User], *server.Response[server.ErrorResponse]) {
	if err := verifyPhoneNumberAndUsername(req.Data.PhoneNumber, req.Data.PhoneNumberHash, ""); err != nil {
		return nil, err
	}
	usr := buildUserForCreation(req)
	if err := s.usersProcessor.CreateUser(ctx, usr, req.ClientIP); err != nil {
		err = errors.Wrapf(err, "failed to create user %#v", req.Data)
		switch {
		case errors.Is(err, users.ErrDuplicate):
			if tErr := terror.As(err); tErr != nil {
				return nil, server.Conflict(err, duplicateUserErrorCode, tErr.Data)
			}

			fallthrough
		default:
			return nil, server.Unexpected(err)
		}
	}
	idMetadataField := auth.FirebaseIDClaim
	if req.AuthenticatedUser.IsIce() {
		idMetadataField = auth.IceIDClaim
	}
	md := users.JSON(map[string]any{
		auth.RegisteredWithProviderClaim: req.AuthenticatedUser.Provider,
		idMetadataField:                  req.AuthenticatedUser.UserID,
	})
	_, err := s.authEmailLinkClient.UpdateMetadata(ctx, usr.ID, &md)
	if err != nil {
		return nil, server.Unexpected(errors.Wrapf(err, "failed to update auth metadata for:%#v", usr))
	}
	usr.HashCode = 0

	return server.Created(&User{User: usr, Checksum: usr.Checksum()}), nil
}

func buildUserForCreation(req *server.Request[CreateUserRequestBody, User]) *users.User {
	usr := new(users.User)
	usr.ID = req.AuthenticatedUser.UserID
	usr.Email = req.Data.Email
	usr.PhoneNumber = req.Data.PhoneNumber
	usr.PhoneNumberHash = req.Data.PhoneNumberHash
	usr.FirstName = &req.Data.FirstName
	usr.LastName = &req.Data.LastName
	usr.ClientData = req.Data.ClientData
	usr.Language = req.Data.Language

	return usr
}

// ModifyUser godoc
//
//	@Schemes
//	@Description	Modifies an user account
//	@Tags			Accounts
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			Authorization		header		string					true	"Insert your access token"		default(Bearer <Add access token here>)
//	@Param			X-Account-Metadata	header		string					false	"Insert your metadata token"	default(<Add metadata token here>)
//	@Param			userId				path		string					true	"ID of the user"
//	@Param			multiPartFormData	formData	ModifyUserRequestBody	true	"Request params"
//	@Param			profilePicture		formData	file					false	"The new profile picture for the user"
//	@Success		200					{object}	ModifyUserResponse
//	@Failure		400					{object}	server.ErrorResponse	"if validations fail or user for modification email is blocked"
//	@Failure		401					{object}	server.ErrorResponse	"if not authorized"
//	@Failure		403					{object}	server.ErrorResponse	"not allowed"
//	@Failure		404					{object}	server.ErrorResponse	"user is not found; or the referred by is not found"
//	@Failure		409					{object}	server.ErrorResponse	"if username, email or phoneNumber conflict with another user's"
//	@Failure		422					{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500					{object}	server.ErrorResponse
//	@Failure		504					{object}	server.ErrorResponse	"if request times out"
//	@Router			/users/{userId} [PATCH].
func (s *service) ModifyUser( //nolint:gocritic,funlen // .
	ctx context.Context,
	req *server.Request[ModifyUserRequestBody, ModifyUserResponse],
) (*server.Response[ModifyUserResponse], *server.Response[server.ErrorResponse]) {
	if err := validateModifyUser(ctx, req); err != nil {
		return nil, err
	}
	usr := buildUserForModification(req)
	var err error
	var loginSession string
	if usr.Email, loginSession, err = s.emailUpdateRequested(ctx, &req.AuthenticatedUser, usr.Email); err != nil {
		switch {
		case errors.Is(err, users.ErrNotFound):
			return nil, server.NotFound(errors.Wrapf(err, "user with id `%v` was not found", req.AuthenticatedUser.UserID), userNotFoundErrorCode)
		case errors.Is(err, emaillink.ErrUserBlocked):
			return nil, server.BadRequest(err, userBlockedErrorCode)
		case errors.Is(err, emaillink.ErrUserDuplicate):
			return nil, server.Conflict(err, duplicateUserErrorCode)
		default:
			return nil, server.Unexpected(errors.Wrapf(err, "failed to trigger email modification for request:%#v", req.Data))
		}
	}
	err = s.usersProcessor.ModifyUser(users.ContextWithChecksum(ctx, req.Data.Checksum), usr, req.Data.ProfilePicture)
	if err != nil {
		err = errors.Wrapf(err, "failed to modify user for %#v", req.Data)
		switch {
		case errors.Is(err, users.ErrRaceCondition):
			return nil, server.BadRequest(err, raceConditionErrorCode)
		case errors.Is(err, users.ErrRelationNotFound):
			return nil, server.NotFound(err, referralNotFoundErrorCode)
		case errors.Is(err, users.ErrNotFound):
			return nil, server.NotFound(err, userNotFoundErrorCode)
		case errors.Is(err, users.ErrInvalidCountry):
			return nil, server.BadRequest(errors.Errorf("invalid country %v", req.Data.Country), invalidPropertiesErrorCode)
		case errors.Is(err, users.ErrDuplicate):
			if tErr := terror.As(err); tErr != nil {
				return nil, server.Conflict(err, duplicateUserErrorCode, tErr.Data)
			}

			fallthrough
		default:
			return nil, server.Unexpected(err)
		}
	}

	return server.OK(&ModifyUserResponse{User: &User{User: usr, Checksum: usr.Checksum()}, LoginSession: loginSession}), nil
}

func validateModifyUser(ctx context.Context, req *server.Request[ModifyUserRequestBody, ModifyUserResponse]) *server.Response[server.ErrorResponse] {
	if err := req.Data.verifyIfAtLeastOnePropertyProvided(); err != nil {
		return err
	}
	if err := verifyPhoneNumberAndUsername(req.Data.PhoneNumber, req.Data.PhoneNumberHash, req.Data.Username); err != nil {
		return err
	}
	if strings.EqualFold(req.AuthenticatedUser.UserID, req.Data.ReferredBy) {
		return server.UnprocessableEntity(errors.New("you cannot use yourself as your own referral"), invalidPropertiesErrorCode)
	}
	if req.Data.ClientData != nil {
		r := make(users.JSON)
		if err := json.UnmarshalContext(ctx, []byte(*req.Data.ClientData), &r); err != nil {
			return server.UnprocessableEntity(errors.Wrap(err, "`clientData` has to be a json structure"), invalidPropertiesErrorCode)
		}
		req.Data.clientData = &r
	}

	return validateHiddenProfileElements(req)
}

func (s *service) emailUpdateRequested(
	ctx context.Context,
	loggedInUser *server.AuthenticatedUser,
	newEmail string,
) (emailForUpdate, loginSession string, err error) {
	if newEmail == "" || newEmail == loggedInUser.Email {
		return "", "", nil
	}
	// User uses firebase.
	if loggedInUser.Token.IsFirebase() {
		return newEmail, "", nil
	}
	deviceID := loggedInUser.Claims[deviceIDTokenClaim].(string) //nolint:errcheck,forcetypeassert // .
	language := loggedInUser.Language
	if language == "" {
		var oldUser *users.UserProfile
		oldUser, err = s.usersProcessor.GetUserByID(ctx, loggedInUser.UserID)
		if err != nil {
			return "", "", errors.Wrapf(err, "get user %v failed: no language", loggedInUser.UserID)
		}
		language = oldUser.Language
	}

	if loginSession, err = s.authEmailLinkClient.SendSignInLinkToEmail(
		users.ConfirmedEmailContext(ctx, loggedInUser.Email),
		newEmail, deviceID, language,
	); err != nil {
		return "", "", errors.Wrapf(err, "can't send sign in link to email:%v", newEmail)
	}

	return "", loginSession, nil
}

func validateHiddenProfileElements(req *server.Request[ModifyUserRequestBody, ModifyUserResponse]) *server.Response[server.ErrorResponse] {
	if req.Data.HiddenProfileElements == nil {
		return nil
	}
	var invalidHiddenProfileElement *users.HiddenProfileElement
	hiddenProfileElements := *req.Data.HiddenProfileElements
	for i, actual := range hiddenProfileElements {
		var valid bool
		for _, expected := range users.HiddenProfileElements {
			if strings.EqualFold(string(actual), string(expected)) {
				hiddenProfileElements[i] = expected
				valid = true

				break
			}
		}
		if !valid {
			invalidHiddenProfileElement = &actual //nolint:gosec,revive,exportloopref // Its safe. Its the last iteration.

			break
		}
	}
	if invalidHiddenProfileElement != nil {
		err := errors.Errorf("hiddenProfileElement '%v' is invalid, valid values are %#v", *invalidHiddenProfileElement, users.HiddenProfileElements)

		return server.UnprocessableEntity(err, invalidPropertiesErrorCode)
	}

	return nil
}

func buildUserForModification(req *server.Request[ModifyUserRequestBody, ModifyUserResponse]) *users.User { //nolint:funlen // .
	usr := new(users.User)
	usr.ID = req.Data.UserID
	usr.ReferredBy = req.Data.ReferredBy
	usr.Country = strings.ToUpper(req.Data.Country)
	usr.City = req.Data.City
	usr.Username = strings.ToLower(req.Data.Username)
	usr.FirstName = &req.Data.FirstName
	usr.LastName = &req.Data.LastName
	usr.PhoneNumber = req.Data.PhoneNumber
	usr.PhoneNumberHash = req.Data.PhoneNumberHash
	usr.Email = req.Data.Email
	usr.AgendaPhoneNumberHashes = &req.Data.AgendaPhoneNumberHashes
	usr.BlockchainAccountAddress = req.Data.BlockchainAccountAddress
	usr.Language = req.Data.Language
	if req.Data.ClearHiddenProfileElements != nil && *req.Data.ClearHiddenProfileElements {
		empty := make(users.Enum[users.HiddenProfileElement], 0, 0) //nolint:gosimple // .
		usr.HiddenProfileElements = &empty
	} else {
		usr.HiddenProfileElements = req.Data.HiddenProfileElements
	}
	if req.Data.clientData != nil {
		usr.ClientData = req.Data.clientData
	}
	if req.Data.ResetProfilePicture != nil && *req.Data.ResetProfilePicture {
		req.Data.ProfilePicture = new(multipart.FileHeader)
		req.Data.ProfilePicture.Header = textproto.MIMEHeader{"Reset": []string{"true"}}
	}
	if strings.TrimSpace(req.Data.ReferredBy) != "" {
		log.Info(fmt.Sprintf("user(id:`%v`,email:`%v`) attempted to set referredBy to `%v`",
			req.AuthenticatedUser.UserID, req.AuthenticatedUser.Email, req.Data.ReferredBy))
	}
	if strings.TrimSpace(req.Data.Username) != "" {
		log.Info(fmt.Sprintf("user(id:`%v`,email:`%v`) attempted to set username to `%v`",
			req.AuthenticatedUser.UserID, req.AuthenticatedUser.Email, req.Data.Username))
	}

	return usr
}

//nolint:gocognit,gocyclo,revive,cyclop // Highly doubt it.
func (a *ModifyUserRequestBody) verifyIfAtLeastOnePropertyProvided() *server.Response[server.ErrorResponse] {
	if a.Country == "" &&
		a.City == "" &&
		a.Email == "" &&
		a.FirstName == "" &&
		a.LastName == "" &&
		a.PhoneNumber == "" &&
		a.PhoneNumberHash == "" &&
		a.Username == "" &&
		a.ReferredBy == "" &&
		a.Language == "" &&
		a.AgendaPhoneNumberHashes == "" &&
		a.BlockchainAccountAddress == "" &&
		a.HiddenProfileElements == nil &&
		a.ClearHiddenProfileElements == nil &&
		a.ClientData == nil &&
		a.ProfilePicture == nil &&
		a.ResetProfilePicture == nil {
		return server.UnprocessableEntity(errors.New("modify request without values"), invalidPropertiesErrorCode)
	}

	return nil
}

func verifyPhoneNumberAndUsername(phoneNumber, phoneNumberHash, username string) *server.Response[server.ErrorResponse] {
	if (phoneNumber == "" && phoneNumberHash != "") || (phoneNumberHash == "" && phoneNumber != "") {
		return server.UnprocessableEntity(errors.New("phoneNumber must be provided only together with phoneNumberHash"), invalidPropertiesErrorCode)
	}
	if username != "" && !users.CompiledUsernameRegex.MatchString(username) {
		err := errors.Errorf("username: %v is invalid, it should match regex: %v", username, users.UsernameRegex)

		return server.BadRequest(err, invalidUsernameErrorCode)
	}

	return nil
}

// DeleteUser godoc
//
//	@Schemes
//	@Description	Deletes an user account
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			Authorization		header	string	true	"Insert your access token"		default(Bearer <Add access token here>)
//	@Param			X-Account-Metadata	header	string	false	"Insert your metadata token"	default(<Add metadata token here>)
//	@Param			userId				path	string	true	"ID of the User"
//	@Success		200					"OK - found and deleted"
//	@Success		204					"No Content - already deleted"
//	@Failure		400					{object}	server.ErrorResponse	"if validations fail"
//	@Failure		401					{object}	server.ErrorResponse	"if not authorized"
//	@Failure		403					{object}	server.ErrorResponse	"not allowed"
//	@Failure		422					{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500					{object}	server.ErrorResponse
//	@Failure		504					{object}	server.ErrorResponse	"if request times out"
//	@Router			/users/{userId} [DELETE].
func (s *service) DeleteUser( //nolint:gocritic // False negative.
	ctx context.Context,
	req *server.Request[DeleteUserArg, any],
) (*server.Response[any], *server.Response[server.ErrorResponse]) {
	if err := s.usersProcessor.DeleteUser(ctx, req.Data.UserID); err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return server.NoContent(), nil
		}

		return nil, server.Unexpected(errors.Wrapf(err, "failed to delete user with id: %v", req.Data.UserID))
	}
	if err := server.Auth(ctx).DeleteUser(ctx, req.Data.UserID); err != nil && !errors.Is(err, auth.ErrUserNotFound) {
		return nil, server.Unexpected(errors.Wrapf(err, "failed to delete auth user:%#v", req.Data.UserID))
	}

	return server.OK[any](), nil
}
