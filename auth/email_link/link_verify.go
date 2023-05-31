// SPDX-License-Identifier: ice License 1.0

package emaillink

import (
	"context"
	"fmt"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/auth"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:funlen // .
func (r *repository) FinishLoginUsingMagicLink(ctx context.Context, emailLinkPayload string) (refreshToken, accessToken string, err error) {
	var claims emailClaims
	if err = auth.VerifyJWTCommonFields(emailLinkPayload, r, &claims); err != nil {
		return "", "", errors.Wrapf(err, "invalid email token:%v", emailLinkPayload)
	}
	email := claims.Subject
	now := time.Now()
	user, err := r.getUserByEmail(ctx, email, claims.OldEmail)
	if err != nil {
		if storage.IsErr(err, storage.ErrNotFound) {
			return "", "", errors.Wrapf(ErrNoConfirmationRequired, "no pending confirmation for email:%v", email)
		}

		return "", "", errors.Wrapf(err, "failed to get user info by email:%v(old email:%v)", email, claims.OldEmail)
	}
	if claims.OldEmail != "" {
		if err = r.handleEmailModification(ctx, user.ID, email, claims.OldEmail, claims.NotifyEmail); err != nil {
			return "", "", errors.Wrapf(err, "failed to handle email modification:%v", email)
		}
		user.Email = email
	}
	refreshTokenSeq, err := r.markOTPasUsed(ctx, user.ID, now, email, claims.OTP)
	if err != nil {
		mErr := multierror.Append(errors.Wrapf(err, "failed to mark otp as used for email:%v", email))
		if claims.OldEmail != "" {
			mErr = multierror.Append(mErr,
				errors.Wrapf(r.rollbackEmailModification(ctx, user.ID, claims.OldEmail), "[rollback] rollbackEmailModification failed for userID:%v", user.ID))
		}
		if storage.IsErr(err, storage.ErrNotFound) {
			return "", "", errors.Wrapf(ErrNoConfirmationRequired, "no pending confirmation for email:%v", email)
		}

		return "", "", mErr.ErrorOrNil() //nolint:wrapcheck // .
	}
	refreshToken, accessToken, err = r.generateTokens(now, user, refreshTokenSeq)

	return refreshToken, accessToken, errors.Wrapf(err, "can't generate tokens for userID:%v", user.ID)
}

func (r *repository) Verify() func(token *jwt.Token) (any, error) {
	return func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok || token.Method.Alg() != jwt.SigningMethodHS256.Name {
			return nil, errors.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		if iss, err := token.Claims.GetIssuer(); err != nil || iss != auth.JwtIssuer {
			return nil, errors.Wrapf(ErrInvalidToken, "invalid issuer:%v", iss)
		}

		return []byte(r.cfg.EmailJWTSecret), nil
	}
}

func (r *repository) markOTPasUsed(ctx context.Context, userID users.UserID, now *time.Time, email, otp string) (tokenSeq int64, err error) {
	return r.updateEmailConfirmations(ctx, userID, 0, now, email, nil, otp)
}

//nolint:revive // .
func (r *repository) updateEmailConfirmations(ctx context.Context, userID users.UserID, currentSeq int64,
	now *time.Time, email string, customClaims *users.JSON, otp string,
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
	updatedValue, err := storage.ExecOne[issuedTokenSeq](ctx, r.db, fmt.Sprintf(`
		UPDATE email_confirmations
			SET token_issued_at = $2,
		        user_id = $3,
		        otp = $3,
				issued_token_seq = COALESCE(email_confirmations.issued_token_seq,0) + 1
				%v
		WHERE  (email_confirmations.email = $1) 
		AND   ((email_confirmations.otp = $4) OR
		       (email_confirmations.user_id = $3 AND email_confirmations.issued_token_seq::text = $4::text))
		RETURNING issued_token_seq`, customClaimsClause), params...)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to assign refreshed token to pending email confirmation for params:%#v", params...)
	}

	return updatedValue.IssuedTokenSeq, nil
}
