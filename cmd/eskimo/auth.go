// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/auth"
	"github.com/ice-blockchain/wintr/server"
)

func (s *service) setupAuthRoutes(router *server.Router) {
	router.
		Group("v1r").
		GET("auth/account", server.RootHandler(s.Account))
}

// Account godoc
//
//	@Schemes
//	@Description	Fetches user's account based on token's data
//	@Tags			Auth
//	@Produce		json
//	@Param			Authorization	header		string	true	"Insert your access token"	default(Bearer <Add access token here>)
//	@Success		200				{object}	User
//	@Failure		404				{object}	server.ErrorResponse	"if user do not have an account yet"
//	@Failure		500				{object}	server.ErrorResponse
//	@Failure		504				{object}	server.ErrorResponse	"if request times out"
//	@Router			/auth/account [GET].
func (s *service) Account( //nolint:funlen,gocognit,gocritic,revive // Fallback logic with iceID
	ctx context.Context,
	req *server.Request[GetAccountArg, User],
) (*server.Response[User], *server.Response[server.ErrorResponse]) {
	usr, err := s.usersRepository.GetUserByID(ctx, req.AuthenticatedUser.UserID)
	if err != nil { //nolint:nestif // Fallback logic.
		if errors.Is(err, users.ErrNotFound) {
			iceID, iErr := s.iceClient.IceUserID(ctx, req.AuthenticatedUser.Email)
			if iErr != nil {
				return nil, server.NotFound(multierror.Append(
					errors.Wrapf(err, "user with id `%v` was not found", req.AuthenticatedUser.UserID),
					errors.Wrapf(iErr, "failed to fetch iceID for email `%v` was not found", req.AuthenticatedUser.Email),
				).ErrorOrNil(), userNotFoundErrorCode)
			}
			if iceID != "" {
				const requestingUserIDCtxValueKey = "requestingUserIDCtxValueKey"
				iceIDCtx := context.WithValue(ctx, requestingUserIDCtxValueKey, iceID) //nolint:staticcheck,revive // To bypass profile ownership check.
				usr, iErr = s.usersRepository.GetUserByID(iceIDCtx, iceID)
				if iErr != nil {
					if errors.Is(iErr, users.ErrNotFound) {
						return nil, server.NotFound(multierror.Append(
							errors.Wrapf(err, "user with id '%v'(fb) was not found", req.AuthenticatedUser.UserID),
							errors.Wrapf(iErr, "user with id '%v'(ice) was not found", iceID),
						).ErrorOrNil(), userNotFoundErrorCode)
					}

					return nil, server.Unexpected(err)
				}
				idDoesNotMatch := req.AuthenticatedUser.UserID != usr.ID
				if _, iceIDExists := req.AuthenticatedUser.Token.Claims[auth.IceIDClaim]; idDoesNotMatch && (!iceIDExists && req.AuthenticatedUser.IsFirebase()) {
					if err = server.Auth(ctx).UpdateCustomClaims(ctx, req.AuthenticatedUser.UserID, map[string]any{
						auth.IceIDClaim:                  usr.ID,
						auth.FirebaseIDClaim:             req.AuthenticatedUser.UserID,
						auth.RegisteredWithProviderClaim: auth.ProviderIce,
					}); err != nil {
						return nil, server.Unexpected(err)
					}
				}

				return server.OK(&User{UserProfile: usr, Checksum: usr.Checksum()}), nil
			}

			return nil, server.NotFound(errors.Wrapf(err, "user with id `%v` was not found", req.AuthenticatedUser.UserID), userNotFoundErrorCode)
		}

		return nil, server.Unexpected(errors.Wrapf(err, "failed to get user by id: %v", req.AuthenticatedUser.UserID))
	}

	return server.OK(&User{UserProfile: usr, Checksum: usr.Checksum()}), nil
}
