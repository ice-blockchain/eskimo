// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
)

func (s *service) setupUserValidationRoutes(router *gin.Engine) {
	router.
		Group("v1").
		PUT("user-validations/:userId/phone-number", server.RootHandler(newRequestValidatePhoneNumber, s.ValidatePhoneNumber))
}

// ValidatePhoneNumber godoc
// @Schemes
// @Description  Validates a provided phone number by a one time code previously provided to the user via SMS.
// @Tags         Validations
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string                       true  "Insert your access token"  default(Bearer <Add access token here>)
// @Param        userId         path    string                       true  "ID of the user"
// @Param        request        body    users.PhoneNumberValidation  true  "Request params"
// @Success      200            "ok"
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      403            {object}  server.ErrorResponse  "if not allowed"
// @Failure      404            {object}  server.ErrorResponse  "phone number is not in the process of validation or user not found"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /user-validations/{userId}/phone-number [PUT].
func (s *service) ValidatePhoneNumber(ctx context.Context, r server.ParsedRequest) server.Response {
	if err := s.usersProcessor.ValidatePhoneNumber(ctx, &r.(*RequestValidatePhoneNumber).PhoneNumberValidation); err != nil {
		err = errors.Wrapf(err, "validate phone number failed for %#v", &r.(*RequestValidatePhoneNumber).PhoneNumberValidation)
		switch {
		case errors.Is(err, users.ErrRelationNotFound):
			return *server.NotFound(err, userNotFoundErrorCode)
		case errors.Is(err, users.ErrNotFound):
			return *server.NotFound(err, phoneValidationNotFoundErrorCode)
		case errors.Is(err, users.ErrInvalidPhoneValidationCode):
			return *server.BadRequest(err, invalidValidationCodeErrorCode)
		case errors.Is(err, users.ErrExpiredPhoneValidationCode):
			return *server.BadRequest(err, phoneValidationCodeExpiredErrorCode)
		}

		return server.Unexpected(err)
	}

	return server.OK()
}

func newRequestValidatePhoneNumber() server.ParsedRequest {
	return new(RequestValidatePhoneNumber)
}

func (req *RequestValidatePhoneNumber) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestValidatePhoneNumber) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestValidatePhoneNumber) Validate() *server.Response {
	if req.AuthenticatedUser.ID != req.UserID {
		return server.Forbidden(errors.Errorf("you can only validate your phone numbers. u>%#v!=a>%v", req.UserID, req.AuthenticatedUser.ID))
	}

	return server.RequiredStrings(map[string]string{
		"userId":          req.UserID,
		"phoneNumber":     req.PhoneNumber,
		"phoneNumberHash": req.PhoneNumberHash,
		"validationCode":  req.ValidationCode,
	})
}

func (req *RequestValidatePhoneNumber) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindJSON, c.ShouldBindUri, server.ShouldBindAuthenticatedUser(c)}
}
