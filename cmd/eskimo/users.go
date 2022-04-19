// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"fmt"
	"github.com/ICE-Blockchain/eskimo/users"
	"github.com/ICE-Blockchain/wintr/log"
	"github.com/ICE-Blockchain/wintr/server"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/pkg/errors"
	"net"
	"net/http"
)

func (s *service) setupUserRoutes(router *gin.Engine) {
	router.
		Group("/v1").
		POST("users", server.RootHandler(newRequestCreateUser, s.CreateUser)).
		GET("users/:userId", server.RootHandler(newRequestGetUser, s.GetUser)).
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
//nolint:lll    // @Param        Authorization  header  string  true  "Insert your access token"  default(Bearer <Add access token here>)
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
	if err := s.usersRepository.AddUser(ctx, resp); err != nil {
		if errors.Is(err, users.ErrDuplicate) {
			return server.Response{
				Code: http.StatusConflict,
				Data: server.ErrorResponse{
					Error: err.Error(),
					Code:  userDuplicateCode,
				}.Fail(errors.Wrapf(err, err.Error())),
			}
		}

		return server.Unexpected(err)
	}

	return server.Created(req)
}

func newRequestCreateUser() server.ParsedRequest {
	return new(RequestCreateUser)
}

func (req *RequestCreateUser) user() *users.User {
	return &users.User{
		ID:                req.AuthenticatedUser.ID,
		Email:             req.Email,
		FullName:          req.FullName,
		PhoneNumber:       req.PhoneNumber,
		Username:          req.Username,
		ReferredBy:        req.ReferredBy,
		ProfilePictureURL: defaultUserImage,
		Country:           "TODO: get me based on req.ClientIP using https://www.ip2location.com/development-libraries/ip2location/go",
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

// GetUser godoc
// @Schemes
// @Description  Returns an user account
// @Tags         Accounts
// @Accept       json
// @Produce      json
//nolint:lll    // @Param        Authorization  header    string  true  "Insert your access token"  default(Bearer <Add access token here>)
// @Param        userId             path      string             true   "ID of the user"
// @Success      200                {object}  users.User
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      404            {object}  server.ErrorResponse  "if not found"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /users/{userId} [GET].
func (s *service) GetUser(ctx context.Context, r server.ParsedRequest) server.Response {
	req := r.(*RequestGetUser)
	resp, err := s.usersRepository.GetUser(ctx, req.ID)

	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			m := fmt.Sprintf("user with id `%v` was not found.", req.ID)

			return server.Response{
				Code: http.StatusNotFound,
				Data: server.ErrorResponse{
					Error: m,
					Code:  userNotFoundCode,
				}.Fail(errors.Wrapf(err, m)),
			}
		}
	}

	if req.AuthenticatedUser.ID == req.ID {
		// User is trying to get their own account.
		return server.OK(resp)
	}

	// User is trying to get some other user's account.
	respShort := users.User{
		Username:          resp.Username,
		ProfilePictureURL: resp.ProfilePictureURL,
	}

	return server.OK(respShort)
}

func newRequestGetUser() server.ParsedRequest {
	return new(RequestGetUser)
}

func (req *RequestGetUser) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestGetUser) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestGetUser) Validate() *server.Response {
	return server.RequiredStrings(map[string]string{"userId": req.ID})
}

func (req *RequestGetUser) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindUri, server.ShouldBindAuthenticatedUser(c)}
}

// ModifyUser godoc
// @Schemes
// @Description  Modifies an user account
// @Tags         Accounts
// @Accept       multipart/form-data
// @Produce      json
//nolint:lll    // @Param        Authorization      header    string             true   "Insert your access token"  default(Bearer <Add access token here>)
// @Param        userId         path      string  true  "ID of the user"
// @Param        multiPartFormData  formData  RequestModifyUser  true   "Request params"
// @Param        profilePicture     formData  file               false  "The new profile picture for the user"
// @Success      200            {object}  users.User
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      403                {object}  server.ErrorResponse  "not allowed"
// @Failure      404                {object}  server.ErrorResponse  "user is not found"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /users/{userId} [PATCH].
//nolint:funlen
func (s *service) ModifyUser(ctx context.Context, r server.ParsedRequest) server.Response {
	req := r.(*RequestModifyUser)
	gUser, err := s.usersRepository.GetUser(ctx, req.ID)

	if errors.Is(err, users.ErrNotFound) {
		m := fmt.Sprintf("user with id `%v` was not found.", req.ID)

		return server.Response{
			Code: http.StatusNotFound,
			Data: server.ErrorResponse{
				Error: m,
				Code:  userNotFoundCode,
			}.Fail(errors.Wrapf(err, m)),
		}
	}

	req.ProfilePicture.Filename = fmt.Sprintf("%v", gUser.HashCode)
	err = s.usersRepository.UploadProfilePicture(ctx, &req.ProfilePicture)
	if err != nil {
		return server.Unexpected(err)
	}

	err = s.usersRepository.ModifyUser(ctx, req.user())
	if err != nil {
		return server.Unexpected(err)
	}
	// if user specified a phoneNumber in the request body, then we proceed with phone number confirmation flow:
	// step 0: don`t update the phone number in users table
	// step 1: insert into phone_number_validation_codes // TODO ask Robert about the pattern of the code
	// step 2: use https://www.twilio.com/docs/libraries/go to send SMS with that code to the user`s phone number
	return server.OK(req)
}

func newRequestModifyUser() server.ParsedRequest {
	return new(RequestModifyUser)
}

func (req *RequestModifyUser) user() *users.User {
	return &users.User{
		ID:                req.ID,
		Email:             req.Email,
		FullName:          req.FullName,
		PhoneNumber:       req.PhoneNumber,
		Username:          req.Username,
		ProfilePictureURL: req.ProfilePicture.Filename,
		Country:           "TODO by clients IP",
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

		return &server.Response{
			Code: http.StatusForbidden,
			Data: server.ErrorResponse{
				Error: "only updating your own account is allowed",
				Code:  "NOT_ALLOWED",
			}.Fail(err),
		}
	}

	return nil
}

func (req *RequestModifyUser) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{
		func(obj interface{}) error {
			err := c.ShouldBindWith(obj, binding.FormMultipart)

			return err
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
//nolint:lll    // @Param        Authorization  header  string  true  "Insert your access token"  default(Bearer <Add access token here>)
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

	if err := s.usersRepository.RemoveUser(ctx, req.ID); err != nil {
		if errors.Is(err, users.ErrNotFound) {
			log.Error(errors.Wrap(err, "user not found"), "RequestRemoveUser", req)

			return server.NoContent()
		}

		return server.Unexpected(err)
	}

	return server.OK(req)
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

		return &server.Response{
			Code: http.StatusForbidden,
			Data: server.ErrorResponse{
				Error: "only deleting your own account is allowed",
				Code:  "NOT_ALLOWED",
			}.Fail(err),
		}
	}

	return nil
}

func (req *RequestDeleteUser) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindUri, server.ShouldBindAuthenticatedUser(c)}
}
