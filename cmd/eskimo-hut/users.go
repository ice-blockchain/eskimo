// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"strings"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
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
// @Schemes
// @Description Creates an user account
// @Tags        Accounts
// @Accept      json
// @Produce     json
// @Param       Authorization header   string        true "Insert your access token" default(Bearer <Add access token here>)
// @Param       request       body     CreateUserArg true "Request params"
// @Success     201           {object} users.User
// @Failure     400           {object} server.ErrorResponse "if validations fail"
// @Failure     401           {object} server.ErrorResponse "if not authorized"
// @Failure     409           {object} server.ErrorResponse "user already exists with that ID or with that username"
// @Failure     422           {object} server.ErrorResponse "if syntax fails"
// @Failure     500           {object} server.ErrorResponse
// @Failure     504           {object} server.ErrorResponse "if request times out"
// @Router      /users [POST].
func (s *service) CreateUser( //nolint:funlen,gocritic // .
	ctx context.Context,
	req *server.Request[CreateUserArg, users.User],
) (*server.Response[users.User], *server.Response[server.ErrorResponse]) {
	if err := verifyPhoneNumberAndUsername(req.Data.PhoneNumber, req.Data.PhoneNumberHash, req.Data.Username); err != nil {
		return nil, err
	}
	if strings.EqualFold(req.AuthenticatedUser.ID, req.Data.ReferredBy) {
		return nil, server.UnprocessableEntity(errors.New("you cannot use yourself as your own referral"), invalidPropertiesErrorCode)
	}
	usr := &users.User{
		PublicUserInformation: users.PublicUserInformation{
			ID:          req.AuthenticatedUser.ID,
			Username:    strings.ToLower(req.Data.Username),
			PhoneNumber: req.Data.PhoneNumber,
		},
		Email:           req.Data.Email,
		ReferredBy:      req.Data.ReferredBy,
		PhoneNumberHash: req.Data.PhoneNumberHash,
	}
	if err := s.usersProcessor.CreateUser(ctx, usr, req.ClientIP); err != nil {
		err = errors.Wrapf(err, "failed to create user %#v", req.Data)
		switch {
		case errors.Is(err, users.ErrRelationNotFound):
			return nil, server.NotFound(err, referralNotFoundErrorCode)
		case errors.Is(err, users.ErrDuplicate):
			if tErr := terror.As(err); tErr != nil {
				return nil, server.Conflict(err, duplicateUserErrorCode, tErr.Data)
			}

			fallthrough
		default:
			return nil, server.Unexpected(err)
		}
	}

	return server.Created(usr), nil
}

// ModifyUser godoc
// @Schemes
// @Description Modifies an user account
// @Tags        Accounts
// @Accept      multipart/form-data
// @Produce     json
// @Param       Authorization     header   string        true  "Insert your access token" default(Bearer <Add access token here>)
// @Param       userId            path     string        true  "ID of the user"
// @Param       multiPartFormData formData ModifyUserArg true  "Request params"
// @Param       profilePicture    formData file          false "The new profile picture for the user"
// @Success     200               {object} users.User
// @Failure     400               {object} server.ErrorResponse "if validations fail"
// @Failure     401               {object} server.ErrorResponse "if not authorized"
// @Failure     403               {object} server.ErrorResponse "not allowed"
// @Failure     404               {object} server.ErrorResponse "user is not found"
// @Failure     409               {object} server.ErrorResponse "if username conflicts with another other user's"
// @Failure     422               {object} server.ErrorResponse "if syntax fails"
// @Failure     500               {object} server.ErrorResponse
// @Failure     504               {object} server.ErrorResponse "if request times out"
// @Router      /users/{userId} [PATCH].
func (s *service) ModifyUser( //nolint:funlen,gocritic // .
	ctx context.Context,
	req *server.Request[ModifyUserArg, users.User],
) (*server.Response[users.User], *server.Response[server.ErrorResponse]) {
	if err := req.Data.verifyIfAtLeastOnePropertyProvided(); err != nil {
		return nil, err
	}
	if err := verifyPhoneNumberAndUsername(req.Data.PhoneNumber, req.Data.PhoneNumberHash, req.Data.Username); err != nil {
		return nil, err
	}
	usr := &users.User{
		PublicUserInformation: users.PublicUserInformation{
			ID:          req.Data.UserID,
			FirstName:   req.Data.FirstName,
			LastName:    req.Data.LastName,
			Username:    strings.ToLower(req.Data.Username),
			PhoneNumber: req.Data.PhoneNumber,
			DeviceLocation: users.DeviceLocation{
				Country: strings.ToUpper(req.Data.Country),
				City:    req.Data.City,
			},
		},
		Email:                   req.Data.Email,
		PhoneNumberHash:         req.Data.PhoneNumberHash,
		AgendaPhoneNumberHashes: req.Data.AgendaPhoneNumberHashes,
	}
	if err := s.usersProcessor.ModifyUser(ctx, usr, req.Data.ProfilePicture); err != nil {
		err = errors.Wrapf(err, "failed to modify user for %#v", req.Data)
		switch {
		case errors.Is(err, users.ErrNotFound):
			return nil, server.NotFound(err, userNotFoundErrorCode)
		case errors.Is(err, users.ErrInvalidCountry):
			return nil, server.BadRequest(errors.Errorf("invalid country %v", req.Data.Country), invalidPropertiesErrorCode)
		case errors.Is(err, users.ErrInvalidPhoneNumber):
			return nil, server.BadRequest(err, phoneNumberInvalidErrorCode)
		case errors.Is(err, users.ErrDuplicate):
			if tErr := terror.As(err); tErr != nil {
				return nil, server.Conflict(err, duplicateUserErrorCode, tErr.Data)
			}

			fallthrough
		case errors.Is(err, users.ErrInvalidPhoneNumberFormat):
			if tErr := terror.As(err); tErr != nil {
				return nil, server.BadRequest(err, phoneNumberFormatInvalidErrorCode, tErr.Data)
			}

			fallthrough
		default:
			return nil, server.Unexpected(err)
		}
	}

	return server.OK(usr), nil
}

//nolint:gocognit // Highly doubt it.
func (a *ModifyUserArg) verifyIfAtLeastOnePropertyProvided() *server.Response[server.ErrorResponse] {
	if a.Country == "" &&
		a.City == "" &&
		a.Email == "" &&
		a.FirstName == "" &&
		a.LastName == "" &&
		a.PhoneNumber == "" &&
		a.Username == "" &&
		a.AgendaPhoneNumberHashes == "" &&
		a.ProfilePicture == nil {
		return server.UnprocessableEntity(errors.New("modify request without values"), invalidPropertiesErrorCode)
	}

	return nil
}

func verifyPhoneNumberAndUsername(phoneNumber, phoneNumberHash, username string) *server.Response[server.ErrorResponse] {
	if (phoneNumber == "" && phoneNumberHash != "") || (phoneNumberHash == "" && phoneNumber != "") {
		return server.UnprocessableEntity(errors.New("phoneNumber must be provided only together with phoneNumberHash"), invalidPropertiesErrorCode)
	}
	if !users.CompiledUsernameRegex.MatchString(username) {
		err := errors.Errorf("username: %v is invalid, it should match regex: %v", username, users.UsernameRegex)

		return server.BadRequest(err, invalidUsernameErrorCode)
	}

	return nil
}

// DeleteUser godoc
// @Schemes
// @Description Deletes an user account
// @Tags        Accounts
// @Accept      json
// @Produce     json
// @Param       Authorization header string true "Insert your access token" default(Bearer <Add access token here>)
// @Param       userId        path   string true "ID of the User"
// @Success     200           "OK - found and deleted"
// @Success     204           "No Content - already deleted"
// @Failure     400           {object} server.ErrorResponse "if validations fail"
// @Failure     401           {object} server.ErrorResponse "if not authorized"
// @Failure     403           {object} server.ErrorResponse "not allowed"
// @Failure     422           {object} server.ErrorResponse "if syntax fails"
// @Failure     500           {object} server.ErrorResponse
// @Failure     504           {object} server.ErrorResponse "if request times out"
// @Router      /users/{userId} [DELETE].
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

	return server.OK[any](), nil
}
