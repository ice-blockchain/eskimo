// SPDX-License-Identifier: ice License 1.0

package emaillink

import (
	"context"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/auth"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	time "github.com/ice-blockchain/wintr/time"
)

func (r *repository) generateRefreshToken(now *time.Time, userID users.UserID, email string, seq int64) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.CustomToken{
		RegisteredClaims: &jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(now.Add(r.cfg.RefreshExpirationTime)),
			NotBefore: jwt.NewNumericDate(*now.Time),
			IssuedAt:  jwt.NewNumericDate(*now.Time),
		},
		Email: email,
		Seq:   seq,
	})
	refreshToken, err := token.SignedString([]byte(r.cfg.JWTSecret))

	return refreshToken, errors.Wrapf(err, "failed to generate refresh token for userID:%v, email:%v", userID, email)
}

func (r *repository) RenewTokens(ctx context.Context, previousRefreshToken string, customClaims *users.JSON) (refreshToken, accessToken string, err error) {
	var token auth.CustomToken
	if err = auth.VerifyJWTCommonFields(previousRefreshToken, &token); err != nil {
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
	refreshTokenSeq, err := r.incrementRefreshTokenSeq(ctx, token.Subject, token.Seq, now, customClaims)
	if err != nil {
		if storage.IsErr(err, storage.ErrNotFound) {
			return "", "", errors.Wrapf(ErrInvalidToken, "refreshToken with wrong sequence:%v provided", refreshTokenSeq)
		}

		return "", "", errors.Wrapf(err, "failed to update pending confirmation for email:%v", token.Email)
	}
	refreshToken, accessToken, err = r.generateTokens(now, user, refreshTokenSeq)

	return refreshToken, accessToken, errors.Wrapf(err, "can't generate tokens for userID:%v, email:%v", token.Subject, token.Email)
}

func (r *repository) generateAccessToken(now *time.Time, refreshTokenSeq int64, user *minimalUser) (string, error) {
	var claims *map[string]any
	role := defaultRole
	if user.CustomClaims != nil {
		customClaimsData := map[string]any(*user.CustomClaims)
		if clRole, ok := customClaimsData["role"]; ok {
			role = clRole.(string) //nolint:errcheck,forcetypeassert // We're issuing them
			delete(customClaimsData, "role")
		}
		if len(customClaimsData) > 0 {
			claims = &customClaimsData
		}
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.CustomToken{
		RegisteredClaims: &jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   user.ID,
			ExpiresAt: jwt.NewNumericDate(now.Add(r.cfg.AccessExpirationTime)),
			NotBefore: jwt.NewNumericDate(*now.Time),
			IssuedAt:  jwt.NewNumericDate(*now.Time),
		},
		Email:    user.Email,
		HashCode: user.HashCode,
		Role:     role,
		Seq:      refreshTokenSeq,
		Custom:   claims,
	})
	tokenStr, err := token.SignedString([]byte(r.cfg.JWTSecret))

	return tokenStr, errors.Wrapf(err, "failed to generate access token for userID:%v and email:%v", user.ID, user.Email)
}

func (r *repository) incrementRefreshTokenSeq(
	ctx context.Context,
	userID users.UserID,
	currentSeq int64,
	now *time.Time,
	customClaims *users.JSON,
) (tokenSeq int64, err error) {
	return r.updateEmailConfirmations(ctx, userID, currentSeq, now, "", customClaims, "")
}

func (r *repository) generateTokens(now *time.Time, user *minimalUser, seq int64) (refreshToken, accessToken string, err error) {
	refreshToken, err = r.generateRefreshToken(now, user.ID, user.Email, seq)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to generate jwt refreshToken for userID:%v", user.ID)
	}
	accessToken, err = r.generateAccessToken(now, seq, user)

	return refreshToken, accessToken, errors.Wrapf(err, "failed to generate accessToken for userID:%v", user.ID)
}
