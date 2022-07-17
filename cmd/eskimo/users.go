// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
)

func (s *service) setupUserRoutes(router *gin.Engine) {
	router.
		Group("v1r").
		GET("users", server.RootHandler(newRequestGetUsers, s.GetUsers)).
		GET("users/:userId", server.RootHandler(newRequestGetUserByID, s.GetUserByID)).
		GET("user-views/username", server.RootHandler(newRequestGetUserByUsername, s.GetUserByUsername))
}

// GetUsers godoc
// @Schemes
// @Description  Returns a list of user account based on the provided query parameters.
// @Tags         Accounts
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string  true   "Insert your access token"  default(Bearer <Add access token here>)
// @Param        keyword        query     string  true   "A keyword to look for in the usernames and full names of users"
// @Param        limit          query     uint64  false  "Limit of elements to return. Defaults to 10"
// @Param        offset         query     uint64  false  "Elements to skip before starting to look for"
// @Success      200            {array}   users.RelatableUserProfile
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /users [GET].
func (s *service) GetUsers(ctx context.Context, req *RequestGetUsers) server.Response {
	resp, err := s.usersRepository.GetUsers(ctx, &req.GetUsersArg)
	if err != nil {
		return server.Unexpected(errors.Wrapf(err, "failed to get users by %#v", &req.GetUsersArg))
	}

	return server.OK(resp)
}

func newRequestGetUsers() *RequestGetUsers {
	return new(RequestGetUsers)
}

func (req *RequestGetUsers) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestGetUsers) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestGetUsers) Validate() *server.Response {
	if req.Limit == 0 {
		req.Limit = 10
	}
	req.UserID = req.AuthenticatedUser.ID

	return server.RequiredStrings(map[string]string{"keyword": req.Keyword})
}

func (*RequestGetUsers) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindQuery, server.ShouldBindAuthenticatedUser(c)}
}

// GetUserByID godoc
// @Schemes
// @Description  Returns an user's account.
// @Tags         Accounts
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string  true  "Insert your access token"  default(Bearer <Add access token here>)
// @Param        userId         path      string  true  "ID of the user"
// @Success      200            {object}  users.UserProfile
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      404            {object}  server.ErrorResponse  "if not found"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /users/{userId} [GET].
func (s *service) GetUserByID(ctx context.Context, req *RequestGetUserByID) server.Response {
	userID := req.UserID
	resp, err := s.usersRepository.GetUserProfileByID(ctx, userID)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return *server.NotFound(errors.Wrapf(err, "user with id `%v` was not found", userID), userNotFoundErrorCode)
		}

		return server.Unexpected(errors.Wrapf(err, "failed to get user by id: %v", userID))
	}
	if userID != req.AuthenticatedUser.ID {
		resp.PhoneNumber = ""
	}

	return server.OK(resp)
}

func newRequestGetUserByID() *RequestGetUserByID {
	return new(RequestGetUserByID)
}

func (req *RequestGetUserByID) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestGetUserByID) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestGetUserByID) Validate() *server.Response {
	return server.RequiredStrings(map[string]string{"userId": req.UserID})
}

func (*RequestGetUserByID) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindUri, server.ShouldBindAuthenticatedUser(c)}
}

// GetUserByUsername godoc
// @Schemes
// @Description  Returns public information about an user account based on an username, making sure the username is valid first.
// @Tags         Accounts
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string  true  "Insert your access token"  default(Bearer <Add access token here>)
// @Param        username       query     string  true  "username of the user. It will validate it first"
// @Success      200            {object}  users.UserProfile
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      404            {object}  server.ErrorResponse  "if not found"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /user-views/username [GET].
func (s *service) GetUserByUsername(ctx context.Context, req *RequestGetUserByUsername) server.Response {
	username := req.Username
	resp, err := s.usersRepository.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return *server.NotFound(errors.Wrapf(err, "user with username `%v` was not found", username), userNotFoundErrorCode)
		}

		return server.Unexpected(errors.Wrapf(err, "failed to get user by username: %v", username))
	}

	return server.OK(resp)
}

func newRequestGetUserByUsername() *RequestGetUserByUsername {
	return new(RequestGetUserByUsername)
}

func (req *RequestGetUserByUsername) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestGetUserByUsername) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestGetUserByUsername) Validate() *server.Response {
	req.Username = strings.ToLower(req.Username)
	if !users.CompiledUsernameRegex.MatchString(req.Username) {
		err := errors.Errorf("username: %v is invalid, it should match regex: %v", req.Username, users.UsernameRegex)

		return server.BadRequest(err, invalidUsernameErrorCode)
	}

	return nil
}

func (*RequestGetUserByUsername) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindQuery, server.ShouldBindAuthenticatedUser(c)}
}
