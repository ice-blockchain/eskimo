// SPDX-License-Identifier: ice License 1.0

package emaillink

import (
	"context"
	"fmt"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/time"
)

func (r *repository) IssueRefreshTokenForMagicLink(ctx context.Context, emailLinkPayload string) (string, error) {
	email, otp, err := r.verifyMagicLink(ctx, emailLinkPayload)
	if err != nil {
		return "", errors.Wrapf(err, "failed to verify email link payload")
	}
	var userID users.UserID
	userID, err = r.findOrGenerateUserIDByEmail(ctx, email)
	if err != nil {
		return "", errors.Wrapf(err, "failed to fetch userID")
	}
	now := time.Now()
	refreshTokenSeq, err := r.updateRefreshTokenForUser(ctx, userID, 0, now, email, nil, otp)
	if err != nil {
		if storage.IsErr(err, storage.ErrNotFound) {
			return "", errors.Wrapf(ErrNoConfirmationRequired, "no pending confirmation for email %v", email)
		}

		return "", errors.Wrapf(err, "failed to update issuing token for email: %v", email)
	}

	return r.generateRefreshToken(now, userID, email, refreshTokenSeq)
}

// TODO: move to wintr?
func (r *repository) parseToken(jwtToken string) (userID, email string, seq int64, err error) {
	var claims Token
	if err := verifyJWTCommonFields(jwtToken, r.cfg.JWTSecret, &claims); err != nil {
		return "", "", 0, errors.Wrapf(err, "invalid refresh/access token")
	}

	return claims.Subject, claims.Email, claims.Seq, nil
}

func (r *repository) verifyMagicLink(ctx context.Context, jwtToken string) (email, otp string, err error) {
	var claims emailClaims
	if err = verifyJWTCommonFields(jwtToken, r.cfg.JWTSecret, &claims); err != nil {
		return "", "", errors.Wrapf(err, "invalid email token")
	}
	email = claims.Subject
	otp = claims.OTP

	return email, otp, err
}

func (r *repository) updateRefreshTokenForUser(ctx context.Context, userID users.UserID, currentSeq int64,
	now *time.Time, email string, customClaims *users.JSON, otp ...any,
) (tokenSeq int64, err error) {
	params := []any{email, now.Time, userID}
	if currentSeq > 0 {
		params = append(params, strconv.FormatInt(currentSeq, 10))
	}
	if len(otp) > 0 {
		params = append(params, otp[0])
	}
	customClaimsClause := ""
	if customClaims != nil {
		params = append(params, customClaims)
		customClaimsClause = ",\n\t\t\t\tcustom_claims = $5::jsonb"
	}
	updatedValue, err := storage.ExecOne[issuedTokenSeq](ctx, r.db, fmt.Sprintf(`
		UPDATE pending_email_confirmations
			SET token_issued_at = $2,
		        user_id = $3,
		        otp = $3,
				issued_token_seq = COALESCE(pending_email_confirmations.issued_token_seq,0) + 1
				%v
		WHERE  (pending_email_confirmations.user_id = $3 AND pending_email_confirmations.issued_token_seq::text = $4::text) OR
		         (pending_email_confirmations.email = $1 AND pending_email_confirmations.otp = $4)
		RETURNING issued_token_seq`, customClaimsClause), params...)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to assign refreshed token to pending email confirmation for email %v", email)
	}

	return updatedValue.IssuedTokenSeq, nil
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
