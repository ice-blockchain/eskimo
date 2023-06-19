// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

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
			return errors.Wrapf(ErrNoConfirmationRequired, "[getUserByPk] no pending confirmation for email:%v", email)
		}

		return errors.Wrapf(err, "failed to get user info by email:%v(old email:%v)", email, token.OldEmail)
	}
	if els.Confirmed {
		return errors.Wrapf(ErrNoPendingConfirmation, "no pending confirmation for id:%#v", id)
	}
	if time.Now().After(els.ConfirmationCodeCreatedAt.Add(c.cfg.ConfirmationCode.ExpirationTime)) {
		return errors.Wrapf(ErrConfirmationCodeTimeout, "confirmation code timeout for id:%#v expired", id)
	}
	if els.ConfirmationCodeWrongAttemptsCount >= c.cfg.ConfirmationCode.MaxWrongAttemptsCount {
		return errors.Wrapf(ErrConfirmationCodeAttemptsExceeded, "confirmation code wrong attempts count exceeded for id:%#v", id)
	}
	if els.ConfirmationCode != confirmationCode {
		var mErr *multierror.Error
		if iErr := c.increaseWrongConfirmationCodeAttemptsCount(ctx, &id); iErr != nil {
			mErr = multierror.Append(mErr, errors.Wrapf(iErr,
				"can't increment wrong confirmation code attempts count for email:%v,deviceUniqueID:%v", email, token.DeviceUniqueID))
		}
		mErr = multierror.Append(mErr,
			errors.Wrapf(ErrConfirmationCodeWrong, "wrong confirmation code:%v for emailLinkPayload:%v", confirmationCode, emailLinkPayload))

		return mErr.ErrorOrNil() //nolint:wrapcheck // Not needed.
	}
	if token.OldEmail != "" {
		if err = c.handleEmailModification(ctx, els, email, token.OldEmail, token.NotifyEmail, confirmationCode); err != nil {
			return errors.Wrapf(err, "failed to handle email modification:%v", email)
		}
		els.Email = email
	}
	if fErr := c.finishAuthProcess(ctx, &id, els.UserID, token.OTP); fErr != nil {
		return errors.Wrapf(fErr, "can't finish auth process for userID:%v,email:%v,otp:%v", els.UserID, email, token.OTP)
	}

	return nil
}

func (c *client) increaseWrongConfirmationCodeAttemptsCount(ctx context.Context, id *loginID) error {
	sql := `UPDATE email_link_sign_ins
				SET confirmation_code_wrong_attempts_count = confirmation_code_wrong_attempts_count + 1
			WHERE email = $1
				  AND device_unique_id = $2`
	_, err := storage.Exec(ctx, c.db, sql, id.Email, id.DeviceUniqueID)

	return errors.Wrapf(err,
		"can't update email link sign ins for the user with pk:%#v", id)
}

func (c *client) finishAuthProcess(ctx context.Context, id *loginID, userID, otp string) error {
	confirmed := true
	params := []any{id.Email, time.Now().Time, userID, otp, id.DeviceUniqueID, confirmed}
	sql := `UPDATE email_link_sign_ins
				SET token_issued_at = $2,
					user_id = $3,
					otp = $3,
					issued_token_seq = COALESCE(issued_token_seq, 0) + 1,
					confirmed = $6
			WHERE email_link_sign_ins.email = $1
				  AND otp = $4
				  AND device_unique_id = $5`

	_, err := storage.Exec(ctx, c.db, sql, params...)
	if err != nil {
		return errors.Wrapf(err, "failed to insert generated token data for:%#v", params...)
	}

	return nil
}
