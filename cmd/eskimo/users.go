// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
)

func (s *service) setupUserRoutes(router *gin.Engine) {
	router.
		Group("/v1").
		GET("users/:userId", server.RootHandler(newRequestGetUserByID, s.GetUserByID)).
		GET("user-views/username", server.RootHandler(newRequestGetUserByUsername, s.GetUserByUsername))
}

// GetUserByID godoc
// @Schemes
// @Description  Returns an user account
// @Tags         Accounts
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string  true  "Insert your access token"  default(Bearer <Add access token here>)
// @Param        userId         path      string  true  "ID of the user"
// @Success      200            {object}  users.User
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      404            {object}  server.ErrorResponse  "if not found"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /users/{userId} [GET].
func (s *service) GetUserByID(ctx context.Context, r server.ParsedRequest) server.Response {
	req := r.(*RequestGetUserByID)
	resp, err := s.usersRepository.GetUserByID(ctx, req.ID)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return *server.NotFound(err, userNotFoundCode)
		}

		return server.Unexpected(err)
	}

	if req.AuthenticatedUser.ID == req.ID {
		// User is trying to get their own account.
		return server.OK(resp)
	}
	// User is trying to get some other user's account.
	respShort := users.User{
		ID:                resp.ID,
		Username:          resp.Username,
		ProfilePictureURL: resp.ProfilePictureURL,
	}

	return server.OK(respShort)
}

func newRequestGetUserByID() server.ParsedRequest {
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
	return server.RequiredStrings(map[string]string{"userId": req.ID})
}

func (req *RequestGetUserByID) Bindings(c *gin.Context) []func(obj interface{}) error {
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
// @Success      200            {object}  users.User
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      404            {object}  server.ErrorResponse  "if not found"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /user-views/username [GET].
func (s *service) GetUserByUsername(ctx context.Context, r server.ParsedRequest) server.Response {
	req := r.(*RequestGetUserByUsername)
	resp, err := s.usersRepository.GetUserByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return *server.NotFound(err, userNotFoundCode)
		}

		return server.Unexpected(err)
	}

	return server.OK(resp)
}

func newRequestGetUserByUsername() server.ParsedRequest {
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
	if !compiledUsernameRegex.MatchString(req.Username) {
		err := errors.Errorf("username: %v is invalid, it should match regex: %v", req.Username, usernameRegex)

		return server.BadRequest(err, userInvalidCode)
	}

	return nil
}

func (req *RequestGetUserByUsername) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindQuery, server.ShouldBindAuthenticatedUser(c)}
}
