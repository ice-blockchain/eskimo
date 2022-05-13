// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/countries"
	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
)

func (s *service) setupUserRoutes(router *gin.Engine) {
	router.
		Group("/v1").
		POST("users", server.RootHandler(newRequestCreateUser, s.CreateUser)).
		PATCH("users/:userId", server.RootHandler(newRequestModifyUser, s.ModifyUser)).
		DELETE("users/:userId", server.RootHandler(newRequestDeleteUser, s.DeleteUser))
}

// CreateUser godoc
// @Schemes
// @Description  Creates an user account
// @Tags         Accounts
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string             true  "Insert your access token"  default(Bearer <Add access token here>)
// @Param        request        body      RequestCreateUser  true  "Request params"
// @Success      201            {object}  users.User
// @Failure      400                {object}  server.ErrorResponse  "if validations fail"
// @Failure      401                {object}  server.ErrorResponse  "if not authorized"
// @Failure      409            {object}  server.ErrorResponse  "user already exists with that ID or with that username"
// @Failure      422                {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500                {object}  server.ErrorResponse
// @Failure      504                {object}  server.ErrorResponse  "if request times out"
// @Router       /users [POST].
func (s *service) CreateUser(ctx context.Context, r server.ParsedRequest) server.Response {
	req := r.(*RequestCreateUser)
	resp := req.user()
	resp.Country = s.countriesRepository.Get(ctx, req.ClientIP.String())

	if err := s.usersProcessor.AddUser(ctx, resp); err != nil {
		if errors.Is(err, users.ErrDuplicate) {
			return getServerErrorResponse(http.StatusConflict, err, userDuplicateCode)
		}

		return server.Unexpected(err)
	}

	return server.Created(resp)
}

func newRequestCreateUser() server.ParsedRequest {
	return new(RequestCreateUser)
}

func (req *RequestCreateUser) user() *users.User {
	return &users.User{
		ID:              req.AuthenticatedUser.ID,
		Email:           req.Email,
		FullName:        req.FullName,
		PhoneNumber:     req.PhoneNumber,
		PhoneNumberHash: req.PhoneNumberHash,
		Username:        req.Username,
		ReferredBy:      req.ReferredBy,
	}
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
	return server.RequiredStrings(map[string]string{"username": req.Username})
}

func (req *RequestCreateUser) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindJSON, server.ShouldBindClientIP(c), server.ShouldBindAuthenticatedUser(c)}
}

// ModifyUser godoc
// @Schemes
// @Description  Modifies an user account
// @Tags         Accounts
// @Accept       multipart/form-data
// @Produce      json
// @Param        Authorization      header    string             true   "Insert your access token"  default(Bearer <Add access token here>)
// @Param        userId             path      string             true   "ID of the user"
// @Param        multiPartFormData  formData  RequestModifyUser  true   "Request params"
// @Param        profilePicture     formData  file               false  "The new profile picture for the user"
// @Success      200                {object}  users.User
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      403                {object}  server.ErrorResponse  "not allowed"
// @Failure      404                {object}  server.ErrorResponse  "user is not found"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /users/{userId} [PATCH].
func (s *service) ModifyUser(ctx context.Context, r server.ParsedRequest) server.Response {
	req := r.(*RequestModifyUser)
	user := req.user()

	err := s.usersProcessor.ModifyUser(ctx, user)
	if err != nil {
		err = errors.Wrap(err, "modify user failed")
		switch {
		case errors.Is(err, users.ErrNotFound):
			return getServerErrorResponse(http.StatusNotFound, err, userNotFoundCode)
		case errors.Is(err, users.ErrDuplicate):
			return getServerErrorResponse(http.StatusConflict, err, userDuplicateCode)
		default:
			return server.Unexpected(err)
		}
	}

	return server.OK()
}

func newRequestModifyUser() server.ParsedRequest {
	return new(RequestModifyUser)
}

func (req *RequestModifyUser) user() *users.User {
	return &users.User{
		ID:                      req.ID,
		Email:                   req.Email,
		FullName:                req.FullName,
		PhoneNumber:             req.PhoneNumber,
		PhoneNumberHash:         req.PhoneNumberHash,
		AgendaPhoneNumberHashes: req.AgendaPhoneNumberHashes,
		Username:                req.Username,
		ProfilePicture:          req.ProfilePicture,
		Country:                 req.Country,
	}
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
	if req.ID == "" {
		return server.RequiredStrings(map[string]string{"userId": req.ID})
	}
	if req.ID != req.AuthenticatedUser.ID {
		err := errors.Errorf("update account not allowed for anyone except the owner. "+
			"`%v` tried to update `%v`", req.AuthenticatedUser.ID, req.ID)

		resp := getServerErrorResponse(http.StatusForbidden, err, notAllowed)

		return &resp
	}

	if !req.hasValues() {
		err := errors.New("modify request without values")
		resp := getServerErrorResponse(http.StatusBadRequest, err, userBadRequest)

		return &resp
	}

	if req.Country != "" {
		req.Country = strings.ToLower(req.Country)

		if err := countries.Validate(req.Country); err != nil {
			resp := getServerErrorResponse(http.StatusBadRequest, err, userBadRequest)

			return &resp
		}
	}

	return nil
}

//nolint:gocognit // This is validator of fields
func (req *RequestModifyUser) hasValues() bool {
	if req.Country != "" || req.Email != "" || req.FullName != "" || req.PhoneNumber != "" || req.Username != "" {
		return true
	}
	if req.ProfilePicture.Filename != "" {
		return true
	}
	if req.AgendaPhoneNumberHashes != "" {
		return true
	}

	return false
}

func (req *RequestModifyUser) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{
		func(obj interface{}) error {
			err := c.ShouldBindWith(obj, binding.FormMultipart)
			if err != nil {
				return errors.Wrap(err, "FormMultipart binding failed")
			}

			return nil
		},
		c.ShouldBindUri,
		server.ShouldBindAuthenticatedUser(c),
	}
}

// DeleteUser godoc
// @Schemes
// @Description  Deletes an user account
// @Tags         Accounts
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string  true  "Insert your access token"  default(Bearer <Add access token here>)
// @Param        userId         path    string  true  "ID of the User"
// @Success      200            "OK - found and deleted"
// @Success      204            "No Content - already deleted"
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      403            {object}  server.ErrorResponse  "not allowed"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /users/{userId} [DELETE].
func (s *service) DeleteUser(ctx context.Context, r server.ParsedRequest) server.Response {
	req := r.(*RequestDeleteUser)

	if err := s.usersProcessor.RemoveUser(ctx, req.ID); err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return server.NoContent()
		}

		return server.Unexpected(err)
	}

	return server.OK()
}

func newRequestDeleteUser() server.ParsedRequest {
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
	if req.ID == "" {
		return server.RequiredStrings(map[string]string{"userId": req.ID})
	}
	if req.ID != req.AuthenticatedUser.ID {
		err := errors.Errorf("delete account not allowed for anyone except the owner. "+
			"`%v` tried to delete `%v`", req.AuthenticatedUser.ID, req.ID)

		resp := getServerErrorResponse(http.StatusForbidden, err, notAllowed)

		return &resp
	}

	return nil
}

func (req *RequestDeleteUser) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindUri, server.ShouldBindAuthenticatedUser(c)}
}
