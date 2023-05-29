// SPDX-License-Identifier: ice License 1.0

package emaillink

import (
	"context"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/auth"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	time "github.com/ice-blockchain/wintr/time"
)

func (r *repository) RenewTokens(ctx context.Context, previousRefreshToken string, customClaims *users.JSON) (refreshToken, accessToken string, err error) {
	var token auth.IceToken
	if err = auth.VerifyJWTCommonFields(previousRefreshToken, auth.Secret, &token); err != nil {
		return "", "", errors.Wrapf(err, "failed to verify token:%v", previousRefreshToken)
	}
	now := time.Now()
	user, err := r.getUserByIDOrEmail(ctx, token.Subject, token.Email)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to get user by id:%v", token.Subject)
	}
	if customClaims != nil {
		user.CustomClaims = customClaims
	}
	if user.Email != token.Email {
		return "", "", errors.Wrapf(ErrUserDataMismatch, "user's email:%v does not match token's email:%v", user.Email, token.Email)
	}
	refreshTokenSeq, err := r.incrementRefreshTokenSeq(ctx, token.Subject, token.Email, token.Seq, now, customClaims)
	if err != nil {
		if storage.IsErr(err, storage.ErrNotFound) {
			return "", "", errors.Wrapf(ErrInvalidToken, "refreshToken with wrong sequence:%v provided", refreshTokenSeq)
		}

		return "", "", errors.Wrapf(err, "failed to update pending confirmation for email:%v", token.Email)
	}
	refreshToken, accessToken, err = r.generateTokens(now, user, refreshTokenSeq)

	return refreshToken, accessToken, errors.Wrapf(err, "can't generate tokens for userID:%v, email:%v", token.Subject, token.Email)
}

//nolint:revive // .
func (r *repository) incrementRefreshTokenSeq(
	ctx context.Context,
	userID users.UserID,
	email string,
	currentSeq int64,
	now *time.Time,
	customClaims *users.JSON,
) (tokenSeq int64, err error) {
	return r.updateEmailConfirmations(ctx, userID, currentSeq, now, email, customClaims, "")
}

func (*repository) generateTokens(now *time.Time, user *minimalUser, seq int64) (refreshToken, accessToken string, err error) {
	var claims map[string]any
	if user.CustomClaims != nil {
		claims = *user.CustomClaims
	}

	rt, at, err := auth.GenerateTokens(now, user.ID, user.Email, user.HashCode, seq, claims)

	return rt, at, errors.Wrapf(err, "failed to generate tokens")
}
