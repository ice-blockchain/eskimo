// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"
	"fmt"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:funlen // .
func (c *client) SignIn(ctx context.Context, emailLinkPayload string) (refreshToken, accessToken string, err error) {
	var claims emailClaims
	if err = c.verifyEmailToken(emailLinkPayload, &claims); err != nil {
		return "", "", errors.Wrapf(err, "invalid email token:%v", emailLinkPayload)
	}
	email := claims.Subject
	now := time.Now()
	usr, err := c.getUserByEmail(ctx, email, claims.OldEmail)
	if err != nil {
		if storage.IsErr(err, storage.ErrNotFound) {
			return "", "", errors.Wrapf(ErrNoConfirmationRequired, "[getUserByEmail] no pending confirmation for email:%v", email)
		}

		return "", "", errors.Wrapf(err, "failed to get user info by email:%v(old email:%v)", email, claims.OldEmail)
	}
	if claims.OldEmail != "" {
		if err = c.handleEmailModification(ctx, usr.ID, email, claims.OldEmail, claims.NotifyEmail); err != nil {
			return "", "", errors.Wrapf(err, "failed to handle email modification:%v", email)
		}
		usr.Email = email
	}
	refreshTokenSeq, err := c.markOTPasUsed(ctx, usr.ID, now, email, claims.OTP)
	if err != nil {
		mErr := multierror.Append(errors.Wrapf(err, "failed to mark otp as used for email:%v", email))
		if claims.OldEmail != "" {
			mErr = multierror.Append(mErr,
				errors.Wrapf(c.rollbackEmailModification(ctx, usr.ID, claims.OldEmail), "[rollback] rollbackEmailModification failed for userID:%v", usr.ID))
		}
		if storage.IsErr(err, storage.ErrNotFound) {
			return "", "", errors.Wrapf(ErrNoConfirmationRequired, "[markOTPasUsed] no pending confirmation for email:%v", email)
		}

		return "", "", mErr.ErrorOrNil() //nolint:wrapcheck // .
	}
	refreshToken, accessToken, err = c.generateTokens(now, usr, refreshTokenSeq)

	return refreshToken, accessToken, errors.Wrapf(err, "can't generate tokens for userID:%v", usr.ID)
}

func (c *client) verifyEmailToken(jwtToken string, res jwt.Claims) error {
	if _, err := jwt.ParseWithClaims(jwtToken, res, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok || token.Method.Alg() != jwt.SigningMethodHS256.Name {
			return nil, errors.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		if iss, err := token.Claims.GetIssuer(); err != nil || iss != jwtIssuer {
			return nil, errors.Wrapf(ErrInvalidToken, "invalid issuer:%v", iss)
		}

		return []byte(c.cfg.EmailJWTSecret), nil
	}); err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
			return errors.Wrapf(ErrExpiredToken, "expired or not valid yet token:%v", jwtToken)
		}

		return errors.Wrapf(err, "invalid token:%v", jwtToken)
	}

	return nil
}

func (c *client) markOTPasUsed(ctx context.Context, userID users.UserID, now *time.Time, email, otp string) (tokenSeq int64, err error) {
	return c.updateEmailConfirmations(ctx, userID, email, otp, 0, now, nil)
}

//nolint:revive // .
func (c *client) updateEmailConfirmations(
	ctx context.Context, userID, email, otp string, currentSeq int64, now *time.Time, customClaims *users.JSON,
) (tokenSeq int64, err error) {
	params := []any{email, now.Time, userID}
	if currentSeq > 0 {
		params = append(params, strconv.FormatInt(currentSeq, 10))
	}
	if otp != "" {
		params = append(params, otp)
	}
	customClaimsClause := ""
	if customClaims != nil {
		params = append(params, customClaims)
		customClaimsClause = ",\n\t\t\t\tcustom_claims = $5::jsonb"
	}
	updatedValue, err := storage.ExecOne[issuedTokenSeq](ctx, c.db, fmt.Sprintf(`
		UPDATE email_link_sign_ins
			SET token_issued_at = $2,
		        user_id = $3,
		        otp = $3,
				issued_token_seq = COALESCE(email_link_sign_ins.issued_token_seq,0) + 1
				%v
		WHERE  (email_link_sign_ins.email = $1) 
		AND   ((email_link_sign_ins.otp = $4) OR
		       (email_link_sign_ins.user_id = $3 AND email_link_sign_ins.issued_token_seq::text = $4::text))
		RETURNING issued_token_seq`, customClaimsClause), params...)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to assign refreshed token to pending email confirmation for params:%#v", params...)
	}

	return updatedValue.IssuedTokenSeq, nil
}
