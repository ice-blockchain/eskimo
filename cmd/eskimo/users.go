// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"github.com/ICE-Blockchain/wintr/server"
	"github.com/gin-gonic/gin"
)

func (s *service) setupUserRoutes(router *gin.Engine) {
	router.
		Group("/v1").
		GET("users/:userId", server.RootHandler(newRequestGetUser, s.GetUser))
}

// GetUser godoc
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
func (s *service) GetUser(ctx context.Context, r server.ParsedRequest) server.Response {
	req := r.(*RequestGetUser)

	//TODO implement me
	if req.AuthenticatedUser.ID == req.ID {
		// User is trying to get their own account
	} else {
		// User is trying to get some other user's account
	}

	return server.OK(req)
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
