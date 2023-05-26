// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"
	"strings"

	"github.com/pkg/errors"

	emaillink "github.com/ice-blockchain/eskimo/auth/email_link"
	"github.com/ice-blockchain/wintr/server"
)

func (s *service) setupAuthRoutes(router *server.Router) {
	router.
		Group("v1w").
		POST("auth", server.RootHandler(s.StartEmailLinkAuth)).
		POST("auth/refresh", server.RootHandler(s.RefreshToken)).
		GET("auth/finish", server.RootHandler(s.FinishLoginUsingMagicLink))
}

// StartEmailLinkAuth godoc
//
//	@Schemes
//	@Description	Starts email link auth process
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		StartEmailLinkAuthRequestArg	true	"Request params"
//	@Success		200		{object}	Auth
//	@Failure		422		{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500		{object}	server.ErrorResponse
//	@Failure		504		{object}	server.ErrorResponse	"if request times out"
//	@Router			/auth [POST].
func (s *service) StartEmailLinkAuth( //nolint:gocritic // .
	ctx context.Context,
	req *server.Request[StartEmailLinkAuthRequestArg, Auth],
) (*server.Response[Auth], *server.Response[server.ErrorResponse]) {
	if err := req.Data.verifyIfAtLeastOnePropertyProvided(); err != nil {
		return nil, err
	}
	if err := s.authEmailLinkProcessor.StartEmailLinkAuth(ctx, req.Data.Email); err != nil {
		err = errors.Wrapf(err, "failed to start email link auth %#v", req.Data)
		if err != nil {
			return nil, server.Unexpected(err)
		}
	}

	return server.OK[Auth](), nil
}

func (a *StartEmailLinkAuthRequestArg) verifyIfAtLeastOnePropertyProvided() *server.Response[server.ErrorResponse] {
	if a.Email == "" {
		return server.UnprocessableEntity(errors.New("start email link auth request without email value"), invalidPropertiesErrorCode)
	}

	return nil
}

// FinishLoginUsingMagicLink godoc
//
//	@Schemes
//	@Description	Finishes login flow using magic link
//	@Tags			Auth
//	@Produce		json
//	@Param			token	query		string	true	"Request params"
//	@Success		200		{object}	RefreshedToken
//	@Failure		400		{object}	server.ErrorResponse	"if invalid or expired payload provided"
//	@Failure		404		{object}	server.ErrorResponse	"if email does not need to be confirmed by magic link"
//	@Failure		422		{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500		{object}	server.ErrorResponse
//	@Failure		504		{object}	server.ErrorResponse	"if request times out"
//	@Router			/auth/finish [GET].
func (s *service) FinishLoginUsingMagicLink( //nolint:gocritic // .
	ctx context.Context,
	req *server.Request[MagicLinkPayload, RefreshedToken],
) (*server.Response[RefreshedToken], *server.Response[server.ErrorResponse]) {
	refreshToken, accessToken, err := s.authEmailLinkProcessor.FinishLoginUsingMagicLink(ctx, req.Data.EmailToken)
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

	return server.OK(&RefreshedToken{RefreshToken: refreshToken, AccessToken: accessToken}), nil
}

// RefreshToken godoc
//
//	@Schemes
//	@Description	Issues new access token
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string			true	"Insert your access token"	default(Bearer <Add access token here>)
//	@Param			request			body		RefreshToken	true	"Body containing customClaims"
//	@Success		200				{object}	RefreshedToken
//	@Failure		400				{object}	server.ErrorResponse	"if users data from token does not match data in db"
//	@Failure		403				{object}	server.ErrorResponse	"if invalid or expired refresh token provided"
//	@Failure		404				{object}	server.ErrorResponse	"if user not found"
//	@Failure		422				{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500				{object}	server.ErrorResponse
//	@Failure		504				{object}	server.ErrorResponse	"if request times out"
//	@Router			/auth/refresh [POST].
func (s *service) RefreshToken( //nolint:gocritic // .
	ctx context.Context,
	req *server.Request[RefreshToken, RefreshedToken],
) (*server.Response[RefreshedToken], *server.Response[server.ErrorResponse]) {
	tokenPayload := strings.TrimPrefix(req.Data.Token, "Bearer ")
	nextRefreshToken, accessToken, err := s.authEmailLinkProcessor.RenewTokens(ctx, tokenPayload, req.Data.CustomClaims)
	if err != nil {
		switch {
		case errors.Is(err, emaillink.ErrUserDataMismatch):
			return nil, server.BadRequest(err, dataMismatch)
		case errors.Is(err, emaillink.ErrUserNotFound):
			return nil, server.NotFound(err, userNotFound)
		case errors.Is(err, emaillink.ErrExpiredToken):
			return nil, server.Forbidden(err)
		case errors.Is(err, emaillink.ErrInvalidToken):
			return nil, server.Forbidden(err)
		default:
			return nil, server.Unexpected(err)
		}
	}

	return server.OK(&RefreshedToken{RefreshToken: nextRefreshToken, AccessToken: accessToken}), nil
}
