// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
	"github.com/ice-blockchain/wintr/terror"
)

func (s *service) setupUserRoutes(router *gin.Engine) {
	router.
		Group("v1w").
		POST("users", server.RootHandler(newRequestCreateUser, s.CreateUser)).
		PATCH("users/:userId", server.RootHandler(newRequestModifyUser, s.ModifyUser)).
		DELETE("users/:userId", server.RootHandler(newRequestDeleteUser, s.DeleteUser))
}

// CreateUser godoc
// @Schemes
// @Description Creates an user account
// @Tags        Accounts
// @Accept      json
// @Produce     json
// @Param       Authorization header   string              true "Insert your access token" default(Bearer <Add access token here>)
// @Param       request       body     users.CreateUserArg true "Request params"
// @Success     201           {object} users.User
// @Failure     400           {object} server.ErrorResponse "if validations fail"
// @Failure     401           {object} server.ErrorResponse "if not authorized"
// @Failure     409           {object} server.ErrorResponse "user already exists with that ID or with that username"
// @Failure     422           {object} server.ErrorResponse "if syntax fails"
// @Failure     500           {object} server.ErrorResponse
// @Failure     504           {object} server.ErrorResponse "if request times out"
// @Router      /users [POST].
func (s *service) CreateUser(ctx context.Context, req *RequestCreateUser) server.Response {
	if err := s.usersProcessor.CreateUser(ctx, &req.CreateUserArg); err != nil {
		err = errors.Wrapf(err, "failed to create user %#v", req.User)
		switch {
		case errors.Is(err, users.ErrRelationNotFound):
			return *server.NotFound(err, referralNotFoundErrorCode)
		case errors.Is(err, users.ErrDuplicate):
			if tErr := terror.As(err); tErr != nil {
				return *server.Conflict(err, duplicateUserErrorCode, tErr.Data)
			}

			fallthrough
		default:
			return server.Unexpected(err)
		}
	}

	return server.Created(&req.User)
}

func newRequestCreateUser() *RequestCreateUser {
	return new(RequestCreateUser)
}

func (req *RequestCreateUser) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser.ID = user.ID
	}
}

func (req *RequestCreateUser) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestCreateUser) SetClientIP(ip net.IP) {
	if len(req.ClientIP) == 0 {
		req.ClientIP = ip
	}
}

func (req *RequestCreateUser) GetClientIP() net.IP {
	return req.ClientIP
}

func (req *RequestCreateUser) Validate() *server.Response {
	if err := verifyIfPhoneNumberAndHashProvidedTogether(req.PhoneNumber, req.PhoneNumberHash); err != nil {
		return server.BadRequest(err, invalidPropertiesErrorCode)
	}

	if err := server.RequiredStrings(map[string]string{"username": req.Username}); err != nil {
		return err
	}
	if !users.CompiledUsernameRegex.MatchString(req.Username) {
		err := errors.Errorf("username: %v is invalid, it should match regex: %v", req.Username, users.UsernameRegex)

		return server.BadRequest(err, invalidUsernameErrorCode)
	}
	if strings.EqualFold(req.AuthenticatedUser.ID, req.ReferredBy) {
		return server.BadRequest(errors.New("you cannot use yourself as your own referral"), invalidPropertiesErrorCode)
	}
	req.init()

	return nil
}

func (req *RequestCreateUser) init() {
	req.User.ID = req.AuthenticatedUser.ID
	req.User.ReferredBy = req.ReferredBy
	req.User.Username = strings.ToLower(req.Username)
	req.User.PhoneNumber = req.PhoneNumber
	req.User.PhoneNumberHash = req.PhoneNumberHash
	req.User.Email = req.Email
}

func (*RequestCreateUser) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindJSON, server.ShouldBindClientIP(c), server.ShouldBindAuthenticatedUser(c)}
}

// ModifyUser godoc
// @Schemes
// @Description Modifies an user account
// @Tags        Accounts
// @Accept      multipart/form-data
// @Produce     json
// @Param       Authorization     header   string              true  "Insert your access token" default(Bearer <Add access token here>)
// @Param       userId            path     string              true  "ID of the user"
// @Param       multiPartFormData formData users.ModifyUserArg true  "Request params"
// @Param       profilePicture    formData file                false "The new profile picture for the user"
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
func (s *service) ModifyUser(ctx context.Context, req *RequestModifyUser) server.Response {
	if err := s.usersProcessor.ModifyUser(ctx, &req.ModifyUserArg); err != nil {
		err = errors.Wrapf(err, "failed to modify user for %#v", req.User)
		switch {
		case errors.Is(err, users.ErrNotFound):
			return *server.NotFound(err, userNotFoundErrorCode)
		case errors.Is(err, users.ErrInvalidCountry):
			return *server.BadRequest(errors.Errorf("invalid country %v", req.Country), invalidPropertiesErrorCode)
		case errors.Is(err, users.ErrInvalidPhoneNumber):
			return *server.BadRequest(err, phoneNumberInvalidErrorCode)
		case errors.Is(err, users.ErrDuplicate):
			if tErr := terror.As(err); tErr != nil {
				return *server.Conflict(err, duplicateUserErrorCode, tErr.Data)
			}

			fallthrough
		case errors.Is(err, users.ErrInvalidPhoneNumberFormat):
			if tErr := terror.As(err); tErr != nil {
				return *server.BadRequest(err, phoneNumberFormatInvalidErrorCode, tErr.Data)
			}

			fallthrough
		default:
			return server.Unexpected(err)
		}
	}

	return server.OK(&req.User)
}

func newRequestModifyUser() *RequestModifyUser {
	return new(RequestModifyUser)
}

func (req *RequestModifyUser) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestModifyUser) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestModifyUser) Validate() *server.Response {
	if req.UserID == "" {
		return server.RequiredStrings(map[string]string{"userId": req.UserID})
	}
	if req.UserID != req.AuthenticatedUser.ID {
		return server.Forbidden(errors.Errorf("update account not allowed for anyone except the owner. `%v` tried to update `%v`",
			req.AuthenticatedUser.ID, req.UserID))
	}
	if err := verifyIfPhoneNumberAndHashProvidedTogether(req.PhoneNumber, req.PhoneNumberHash); err != nil {
		return server.BadRequest(err, invalidPropertiesErrorCode)
	}
	if !req.hasValues() {
		return server.BadRequest(errors.New("modify request without values"), invalidPropertiesErrorCode)
	}
	if req.Username != "" && !users.CompiledUsernameRegex.MatchString(req.Username) {
		err := errors.Errorf("username: %v is invalid, it should match regex: %v", req.Username, users.UsernameRegex)

		return server.BadRequest(err, invalidUsernameErrorCode)
	}
	req.init()

	return nil
}

func (req *RequestModifyUser) init() {
	req.User.ID = req.UserID
	req.User.Username = strings.ToLower(req.Username)
	req.User.FirstName = req.FirstName
	req.User.LastName = req.LastName
	req.User.PhoneNumber = req.PhoneNumber
	req.User.PhoneNumberHash = req.PhoneNumberHash
	req.User.Country = strings.ToUpper(req.Country)
	req.User.City = req.City
	req.User.Email = req.Email
	req.User.AgendaPhoneNumberHashes = req.AgendaPhoneNumberHashes
}

//nolint:gocognit // Highly doubt it.
func (req *RequestModifyUser) hasValues() bool {
	return req.Country != "" ||
		req.City != "" ||
		req.Email != "" ||
		req.FirstName != "" ||
		req.LastName != "" ||
		req.PhoneNumber != "" ||
		req.Username != "" ||
		req.AgendaPhoneNumberHashes != "" ||
		req.ProfilePicture != nil
}

func (*RequestModifyUser) Bindings(c *gin.Context) []func(obj interface{}) error {
	multipart := func(obj interface{}) error {
		return errors.Wrap(c.ShouldBindWith(obj, binding.FormMultipart), "formMultipart binding failed")
	}

	return []func(obj interface{}) error{multipart, c.ShouldBindUri, server.ShouldBindAuthenticatedUser(c)}
}

func verifyIfPhoneNumberAndHashProvidedTogether(phoneNumber, phoneNumberHash string) error {
	if phoneNumberHash == "" && phoneNumber != "" {
		return errors.New("phoneNumber must be provided only together with phoneNumberHash")
	}
	if phoneNumber == "" && phoneNumberHash != "" {
		return errors.New("phoneNumberHash must be provided only together with phoneNumber")
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
func (s *service) DeleteUser(ctx context.Context, req *RequestDeleteUser) server.Response {
	if err := s.usersProcessor.DeleteUser(ctx, req.UserID); err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return server.NoContent()
		}

		return server.Unexpected(errors.Wrapf(err, "failed to delete user with id: %v", req.UserID))
	}

	return server.OK()
}

func newRequestDeleteUser() *RequestDeleteUser {
	return new(RequestDeleteUser)
}

func (req *RequestDeleteUser) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestDeleteUser) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestDeleteUser) Validate() *server.Response {
	if req.UserID == "" {
		return server.RequiredStrings(map[string]string{"userId": req.UserID})
	}
	if req.UserID != req.AuthenticatedUser.ID {
		return server.Forbidden(errors.Errorf("delete account not allowed for anyone except the owner. `%v` tried to delete `%v`",
			req.AuthenticatedUser.ID, req.UserID))
	}

	return nil
}

func (*RequestDeleteUser) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindUri, server.ShouldBindAuthenticatedUser(c)}
}
