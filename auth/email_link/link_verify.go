// SPDX-License-Identifier: ice License 1.0

package emaillink

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage/v2"
)

func (r *repository) IssueRefreshToken(ctx context.Context, emailLinkPayload string) (string, error) {
	userID, email, err := r.verifyMagicLink(ctx, emailLinkPayload)
	if err != nil {
		return "", errors.Wrapf(err, "failed to verify email link payload")
	}

	return r.generateRefreshToken(userID, email)
}

func (r *repository) generateRefreshToken(userID, email string) (string, error) {
	now := time.Now().In(time.UTC)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Token{
		RegisteredClaims: &jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(now.Add(r.cfg.RefreshExpirationTime)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		EMail: email,
	})

	tokenStr, err := token.SignedString([]byte(r.cfg.JWTSecret))

	return tokenStr, errors.Wrapf(err, "failed to generate refresh token for user %v %v", userID, email)
}

func (r *repository) verifyMagicLink(ctx context.Context, jwtToken string) (userID, email string, err error) {
	email, otp, err := r.parseMagicLinkToken(jwtToken)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to fetch email and otp code from payload")
	}
	hasPendingConf, err := r.deleteEmailConfirmation(ctx, email, otp)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to delete pending confirmation for email:%v", email)
	}
	if !hasPendingConf {
		return "", "", errors.Wrapf(ErrNoConfirmationRequired, "no pending confirmation for email %v", email)
	}

	return r.findOrGenerateUserIDByEmail(ctx, email)
}

func (r *repository) deleteEmailConfirmation(ctx context.Context, email, otp string) (bool, error) {
	rowsDeleted, err := storage.Exec(ctx, r.db, `DELETE FROM pending_email_confirmations WHERE email = $1 AND otp = $2`, email, otp)
	if err != nil {
		return false, errors.Wrapf(err, "failed to delete pending email confirmation for email %v", email)
	}

	return rowsDeleted == 1, nil
}

func (r *repository) parseMagicLinkToken(jwtToken string) (email, otp string, err error) {
	var claims emailClaims
	if err = verifyJWTCommonFields(jwtToken, r.cfg.JWTSecret, &claims); err != nil {
		return "", "", errors.Wrapf(err, "invalid email token")
	}
	return claims.Subject, claims.OTP, nil
}

func verifyJWTCommonFields(jwtToken, secret string, res jwt.Claims) error {
	if _, err := jwt.ParseWithClaims(jwtToken, res, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secret), nil
	}); err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
			return ErrExpiredToken
		}

		return errors.Wrapf(err, "invalid token")
	}
	if iss, err := res.GetIssuer(); err != nil || iss != jwtIssuer {
		return errors.Wrap(ErrInvalidToken, "invalid issuer")
	}

	return nil
}
