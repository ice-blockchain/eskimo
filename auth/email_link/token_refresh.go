// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/auth"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	time "github.com/ice-blockchain/wintr/time"
)

//nolint:funlen // .
func (c *client) RegenerateTokens(ctx context.Context, previousRefreshToken string, customClaims *users.JSON) (refreshToken, accessToken string, err error) {
	token, err := c.authClient.ParseToken(previousRefreshToken)
	if err != nil {
		if errors.Is(err, auth.ErrExpiredToken) {
			return "", "", errors.Wrapf(ErrExpiredToken, "failed to verify due to expired token:%v", previousRefreshToken)
		}
		if errors.Is(err, auth.ErrInvalidToken) {
			return "", "", errors.Wrapf(ErrInvalidToken, "failed to verify due to invalid token:%v", previousRefreshToken)
		}

		return "", "", errors.Wrapf(err, "failed to verify token:%v", previousRefreshToken)
	}
	now := time.Now()
	usr, err := c.getUserByIDOrEmail(ctx, token.Subject, token.Email)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return "", "", errors.Wrapf(ErrUserNotFound, "user with id %v or email %v not found", token.Subject, token.Email)
		}

		return "", "", errors.Wrapf(err, "failed to get user by id:%v", token.Subject)
	}
	if customClaims != nil {
		usr.CustomClaims = customClaims
	}
	if usr.Email != token.Email {
		return "", "", errors.Wrapf(ErrUserDataMismatch, "user's email:%v does not match token's email:%v", usr.Email, token.Email)
	}
	refreshTokenSeq, err := c.incrementRefreshTokenSeq(ctx, token.Subject, token.Email, token.Seq, now, customClaims)
	if err != nil {
		if storage.IsErr(err, storage.ErrNotFound) {
			return "", "", errors.Wrapf(ErrInvalidToken, "refreshToken with wrong sequence:%v provided", token.Seq)
		}

		return "", "", errors.Wrapf(err, "failed to update pending confirmation for email:%v", token.Email)
	}
	refreshToken, accessToken, err = c.generateTokens(now, usr, refreshTokenSeq)

	return refreshToken, accessToken, errors.Wrapf(err, "can't generate tokens for userID:%v, email:%v", token.Subject, token.Email)
}

//nolint:revive // .
func (c *client) incrementRefreshTokenSeq(
	ctx context.Context,
	userID users.UserID,
	email string,
	currentSeq int64,
	now *time.Time,
	customClaims *users.JSON,
) (tokenSeq int64, err error) {
	return c.updateEmailConfirmations(ctx, userID, email, "", currentSeq, now, customClaims)
}

func (c *client) generateTokens(now *time.Time, usr *minimalUser, seq int64) (refreshToken, accessToken string, err error) {
	var claims map[string]any
	if usr.CustomClaims != nil {
		claims = *usr.CustomClaims
	}
	refreshToken, accessToken, err = c.authClient.GenerateTokens(now, usr.ID, usr.Email, usr.HashCode, seq, claims)
	err = errors.Wrapf(err, "failed to generate tokens for user:%#v", usr)

	return
}
