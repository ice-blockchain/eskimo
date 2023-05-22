// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"

	"github.com/pkg/errors"

	emaillink "github.com/ice-blockchain/eskimo/auth/email_link"
	"github.com/ice-blockchain/wintr/server"
)

// FinishLoginUsingMagicLink godoc
//
//	@Schemes
//	@Description	Finishes login flow using magic link
//	@Tags			Auth
//	@Produce		json
//	@Param			payload	path		string	true	"Request params"
//	@Success		200		{object}	RefreshedToken
//	@Failure		400		{object}	server.ErrorResponse	"if invalid or expired payload provided"
//	@Failure		404		{object}	server.ErrorResponse	"if email does not need to be confirmed by magic link"
//	@Failure		422		{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500		{object}	server.ErrorResponse
//	@Failure		504		{object}	server.ErrorResponse	"if request times out"
//	@Router			/auth/finish/{payload} [GET].
func (s *service) FinishLoginUsingMagicLink( //nolint:gocritic // .
	ctx context.Context,
	req *server.Request[MagicLinkPayload, RefreshedToken],
) (*server.Response[RefreshedToken], *server.Response[server.ErrorResponse]) {
	refreshToken, err := s.authEmailLinkProcessor.IssueRefreshToken(ctx, req.Data.JWTPayload)
	if err != nil {
		err = errors.Wrapf(err, "finish login using magic link failed for %#v", req.Data)
		switch {
		case errors.Is(err, emaillink.ErrExpiredToken):
			return nil, server.BadRequest(err, linkExpired)
		case errors.Is(err, emaillink.ErrInvalidToken):
			return nil, server.BadRequest(err, invalidOTPCode)
		case errors.Is(err, emaillink.ErrNoConfirmationRequired):
			return nil, server.NotFound(err, emailValidationNotFound)
		default:
			return nil, server.Unexpected(err)
		}
	}
	accessToken := "" // TODO implement later.

	return server.OK(&RefreshedToken{RefreshToken: refreshToken, AccessToken: accessToken}), nil
}
