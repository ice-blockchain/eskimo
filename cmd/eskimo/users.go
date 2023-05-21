// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"
	"strings"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
)

func (s *service) setupUserRoutes(router *server.Router) {
	router.
		Group("v1r").
		GET("users", server.RootHandler(s.GetUsers)).
		GET("users/:userId", server.RootHandler(s.GetUserByID)).
		GET("user-views/username", server.RootHandler(s.GetUserByUsername))
}

// GetUsers godoc
//
//	@Schemes
//	@Description	Returns a list of user account based on the provided query parameters.
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Insert your access token"	default(Bearer <Add access token here>)
//	@Param			keyword			query		string	true	"A keyword to look for in the usernames and full names of users"
//	@Param			limit			query		uint64	false	"Limit of elements to return. Defaults to 10"
//	@Param			offset			query		uint64	false	"Elements to skip before starting to look for"
//	@Success		200				{array}		users.MinimalUserProfile
//	@Failure		400				{object}	server.ErrorResponse	"if validations fail"
//	@Failure		401				{object}	server.ErrorResponse	"if not authorized"
//	@Failure		422				{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500				{object}	server.ErrorResponse
//	@Failure		504				{object}	server.ErrorResponse	"if request times out"
//	@Router			/users [GET].
func (s *service) GetUsers( //nolint:gocritic // False negative.
	ctx context.Context,
	req *server.Request[GetUsersArg, []*users.MinimalUserProfile],
) (*server.Response[[]*users.MinimalUserProfile], *server.Response[server.ErrorResponse]) {
	key := string(everythingNotAllowedInUsernamePattern.ReplaceAll([]byte(strings.ToLower(req.Data.Keyword)), []byte("")))
	if key == "" || !strings.EqualFold(key, req.Data.Keyword) {
		err := errors.Errorf("username: %v is invalid, it should match regex: %v", req.Data.Keyword, everythingNotAllowedInUsernamePattern)

		return nil, server.BadRequest(err, invalidKeywordErrorCode)
	}
	if req.Data.Limit == 0 {
		req.Data.Limit = 10
	}
	resp, err := s.usersRepository.GetUsers(ctx, req.Data.Keyword, req.Data.Limit, req.Data.Offset)
	if err != nil {
		return nil, server.Unexpected(errors.Wrapf(err, "failed to get users by %#v", req.Data))
	}

	return server.OK(&resp), nil
}

// GetUserByID godoc
//
//	@Schemes
//	@Description	Returns an user's account.
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Insert your access token"	default(Bearer <Add access token here>)
//	@Param			userId			path		string	true	"ID of the user"
//	@Success		200				{object}	User
//	@Failure		400				{object}	server.ErrorResponse	"if validations fail"
//	@Failure		401				{object}	server.ErrorResponse	"if not authorized"
//	@Failure		404				{object}	server.ErrorResponse	"if not found"
//	@Failure		422				{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500				{object}	server.ErrorResponse
//	@Failure		504				{object}	server.ErrorResponse	"if request times out"
//	@Router			/users/{userId} [GET].
func (s *service) GetUserByID( //nolint:gocritic // False negative.
	ctx context.Context,
	req *server.Request[GetUserByIDArg, User],
) (*server.Response[User], *server.Response[server.ErrorResponse]) {
	usr, err := s.usersRepository.GetUserByID(ctx, req.Data.UserID)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return nil, server.NotFound(errors.Wrapf(err, "user with id `%v` was not found", req.Data.UserID), userNotFoundErrorCode)
		}

		return nil, server.Unexpected(errors.Wrapf(err, "failed to get user by id: %v", req.Data.UserID))
	}

	return server.OK(&User{UserProfile: usr, Checksum: usr.Checksum()}), nil
}

// GetUserByUsername godoc
//
//	@Schemes
//	@Description	Returns public information about an user account based on an username, making sure the username is valid first.
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	true	"Insert your access token"	default(Bearer <Add access token here>)
//	@Param			username		query		string	true	"username of the user. It will validate it first"
//	@Success		200				{object}	users.UserProfile
//	@Failure		400				{object}	server.ErrorResponse	"if validations fail"
//	@Failure		401				{object}	server.ErrorResponse	"if not authorized"
//	@Failure		404				{object}	server.ErrorResponse	"if not found"
//	@Failure		422				{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500				{object}	server.ErrorResponse
//	@Failure		504				{object}	server.ErrorResponse	"if request times out"
//	@Router			/user-views/username [GET].
func (s *service) GetUserByUsername( //nolint:gocritic // False negative.
	ctx context.Context,
	req *server.Request[GetUserByUsernameArg, users.UserProfile],
) (*server.Response[users.UserProfile], *server.Response[server.ErrorResponse]) {
	if !users.CompiledUsernameRegex.MatchString(req.Data.Username) {
		err := errors.Errorf("username: %v is invalid, it should match regex: %v", req.Data.Username, users.UsernameRegex)

		return nil, server.BadRequest(err, invalidUsernameErrorCode)
	}

	resp, err := s.usersRepository.GetUserByUsername(ctx, strings.ToLower(req.Data.Username))
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return nil, server.NotFound(errors.Wrapf(err, "user with username `%v` was not found", req.Data.Username), userNotFoundErrorCode)
		}

		return nil, server.Unexpected(errors.Wrapf(err, "failed to get user by username: %v", req.Data.Username))
	}

	return server.OK(resp), nil
}
