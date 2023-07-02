// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/auth"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:funlen // .
func (c *client) SignIn(ctx context.Context, emailLinkPayload, confirmationCode string) error {
	var token magicLinkToken
	if err := parseJwtToken(emailLinkPayload, c.cfg.EmailValidation.JwtSecret, &token); err != nil {
		return errors.Wrapf(err, "invalid email token:%v", emailLinkPayload)
	}
	email := token.Subject
	id := loginID{Email: email, DeviceUniqueID: token.DeviceUniqueID}
	els, err := c.getEmailLinkSignInByPk(ctx, &id, token.OldEmail)
	if err != nil {
		if storage.IsErr(err, storage.ErrNotFound) {
			return errors.Wrapf(ErrNoConfirmationRequired, "[getEmailLinkSignInByPk] no pending confirmation for email:%v", email)
		}

		return errors.Wrapf(err, "failed to get user info by email:%v(old email:%v)", email, token.OldEmail)
	}
	if els.OTP == *els.UserID {
		return errors.Wrapf(ErrNoConfirmationRequired, "no pending confirmation for email:%v", email)
	}
	if els.ConfirmationCodeWrongAttemptsCount >= c.cfg.ConfirmationCode.MaxWrongAttemptsCount {
		return errors.Wrapf(ErrConfirmationCodeAttemptsExceeded, "confirmation code wrong attempts count exceeded for id:%#v", id)
	}
	if els.ConfirmationCode != confirmationCode {
		var shouldBeBlocked bool
		if els.ConfirmationCodeWrongAttemptsCount+1 == c.cfg.ConfirmationCode.MaxWrongAttemptsCount {
			shouldBeBlocked = true
		}
		var mErr *multierror.Error
		if iErr := c.increaseWrongConfirmationCodeAttemptsCount(ctx, &id, shouldBeBlocked); iErr != nil {
			mErr = multierror.Append(mErr, errors.Wrapf(iErr,
				"can't increment wrong confirmation code attempts count for email:%v,deviceUniqueID:%v", email, token.DeviceUniqueID))
		}
		mErr = multierror.Append(mErr,
			errors.Wrapf(ErrConfirmationCodeWrong, "wrong confirmation code:%v for emailLinkPayload:%v", confirmationCode, emailLinkPayload))

		return mErr.ErrorOrNil() //nolint:wrapcheck // Not needed.
	}
	var emailConfirmed bool
	if token.OldEmail != "" {
		if err = c.handleEmailModification(ctx, els, email, token.OldEmail, token.NotifyEmail); err != nil {
			return errors.Wrapf(err, "failed to handle email modification:%v", email)
		}
		emailConfirmed = true
		els.Email = email
	}
	if fErr := c.finishAuthProcess(ctx, &id, *els.UserID, token.OTP, els.IssuedTokenSeq, emailConfirmed); fErr != nil {
		return errors.Wrapf(fErr, "can't finish auth process for userID:%v,email:%v,otp:%v", els.UserID, email, token.OTP)
	}

	return nil
}

//nolint:revive // Not to create duplicated function with/without bool flag.
func (c *client) increaseWrongConfirmationCodeAttemptsCount(ctx context.Context, id *loginID, shouldBeBlocked bool) error {
	params := []any{id.Email, id.DeviceUniqueID}
	var blockSQL string
	if shouldBeBlocked {
		blockSQL = ",blocked_until = $3"
		params = append(params, time.Now().Add(c.cfg.EmailValidation.BlockDuration))
	}
	sql := fmt.Sprintf(`UPDATE email_link_sign_ins
				SET confirmation_code_wrong_attempts_count = confirmation_code_wrong_attempts_count + 1
				%v
			WHERE email = $1
				  AND device_unique_id = $2`, blockSQL)
	_, err := storage.Exec(ctx, c.db, sql, params...)

	return errors.Wrapf(err, "can't update email link sign ins for the user with pk:%#v", id)
}

//nolint:revive // We need them to reduce write load.
func (c *client) finishAuthProcess(ctx context.Context, id *loginID, userID, otp string, issuedTokenSeq int64, emailConfirmed bool) error {
	emailConfirmedAt := "null"
	if emailConfirmed {
		emailConfirmedAt = "$2"
	}
	params := []any{id.Email, time.Now().Time, userID, otp, id.DeviceUniqueID, issuedTokenSeq}
	sql := fmt.Sprintf(`UPDATE email_link_sign_ins
				SET token_issued_at = $2,
					user_id = $3,
					otp = $3,
					email_confirmed_at = %[5]v,
					issued_token_seq = COALESCE(issued_token_seq, 0) + 1,
				    custom_claims = (COALESCE(email_link_sign_ins.custom_claims,'{}'::jsonb)||(CASE 
				   						 WHEN (SELECT id FROM users WHERE id = $3) = $3 AND (SELECT user_id FROM email_link_sign_ins WHERE user_id = $3 LIMIT 1) is NULL 
											THEN jsonb_build_object('%[1]v', $3, '%[2]v', '%[3]v')
				    				ELSE jsonb_build_object('%[2]v', '%[4]v') END))
			WHERE email_link_sign_ins.email = $1
				  AND otp = $4
				  AND device_unique_id = $5
				  AND issued_token_seq = $6`, auth.FirebaseIDClaim, auth.RegisteredWithProviderClaim, auth.ProviderFirebase, auth.ProviderIce, emailConfirmedAt)

	_, err := storage.Exec(ctx, c.db, sql, params...)
	if err != nil {
		return errors.Wrapf(err, "failed to insert generated token data for:%#v", params...)
	}

	return nil
}
