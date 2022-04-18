// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"github.com/ICE-Blockchain/wintr/server"
	"github.com/gin-gonic/gin"
	"net/http"
	"regexp"
)

func (s *service) setupUserValidationRoutes(router *gin.Engine) {
	router.
		Group("/v1").
		PUT("user-validations/username", server.RootHandler(newRequestValidateUsername, s.ValidateUsername)).
		PUT("user-validations/phone-number", server.RootHandler(newRequestValidatePhoneNumber, s.ValidatePhoneNumber))
}

// ValidateUsername godoc
// @Schemes
// @Description  Validates a provided username
// @Tags         Validations
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string                   true  "Insert your access token"  default(Bearer <Add access token here>)
// @Param        request        body    RequestValidateUsername  true  "Request params"
// @Success      200            "username is ok and can be used"
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      409            {object}  server.ErrorResponse  "user exists"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /user-validations/username [PUT].
func (s *service) ValidateUsername(ctx context.Context, r server.ParsedRequest) server.Response {
	req := r.(*RequestValidateUsername)

	eval := regexp.MustCompile(`[\w\-.]+`)

	if len(req.Username) < 4 || len(req.Username) > 20 || eval.MatchString(req.Username) == false {
		return server.Response{
			Code: http.StatusBadRequest,
			Data: server.ErrorResponse{
				Error: "username incorrect",
				Code:  "NOT_ALLOWED",
			},
		}
	}

	return server.OK(req)
}

func newRequestValidateUsername() server.ParsedRequest {
	return new(RequestValidateUsername)
}

func (req *RequestValidateUsername) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestValidateUsername) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestValidateUsername) Validate() *server.Response {
	// TODO also validate structure of the username
	return server.RequiredStrings(map[string]string{"username": req.Username})
}

func (req *RequestValidateUsername) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindJSON, server.ShouldBindAuthenticatedUser(c)}
}

// ValidatePhoneNumber godoc
// @Schemes
// @Description  Validates a provided phone number by a one time code previously provided to the user via SMS.
// @Tags         Validations
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string                      true  "Insert your access token"  default(Bearer <Add access token here>)
// @Param        request        body    RequestValidatePhoneNumber  true  "Request params"
// @Success      200            "ok"
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      404            {object}  server.ErrorResponse  "phone number is not in the process of validation"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /user-validations/phone-number [PUT].
func (s *service) ValidatePhoneNumber(ctx context.Context, r server.ParsedRequest) server.Response {
	req := r.(*RequestValidatePhoneNumber)

	//TODO implement me
	// if phone number & code match the ones in the database => remove the entry and return 200
	// if entry for that phone number is not in phone_number_validation_codes => 404
	// if entry exists but has wrong code => 400 INVALID_CODE

	return server.OK(req)
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
	return server.RequiredStrings(map[string]string{"phoneNumber": req.PhoneNumber, "validationCode": req.ValidationCode})
}

func (req *RequestValidatePhoneNumber) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindJSON, server.ShouldBindAuthenticatedUser(c)}
}
