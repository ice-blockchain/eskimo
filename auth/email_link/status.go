// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage/v2"
)

func (c *client) Status(ctx context.Context, loginSession string) (tokens *Tokens, err error) {
	var token loginFlowToken
	if err = parseJwtToken(loginSession, c.cfg.LoginSession.JwtSecret, &token); err != nil {
		return nil, errors.Wrapf(err, "can't parse login session:%v", loginSession)
	}
	id := loginID{Email: token.Subject, DeviceUniqueID: token.DeviceUniqueID}
	els, err := c.getConfirmedEmailLinkSignIn(ctx, &id, token.ConfirmationCode)
	if storage.IsErr(err, storage.ErrNotFound) {
		return nil, errors.Wrapf(ErrNoPendingLoginSession, "no pending login session:%v,id:%#v", loginSession, id)
	}
	if els.UserID == nil || els.OTP != *els.UserID {
		return nil, errors.Wrapf(ErrStatusNotVerified, "not verified for id:%#v", id)
	}
	if els.ConfirmationCode == *els.UserID {
		return nil, errors.Wrapf(ErrNoPendingLoginSession, "tokens already provided for id:%#v", id)
	}
	tokens, err = c.generateTokens(els.TokenIssuedAt, els, els.IssuedTokenSeq)
	if err != nil {
		return nil, errors.Wrapf(err, "can't generate tokens for id:%#v", id)
	}
	if rErr := c.resetLoginSession(ctx, &id, els, token.ConfirmationCode); rErr != nil {
		return nil, errors.Wrapf(rErr, "can't reset login session for id:%#v", id)
	}

	return
}

func (c *client) resetLoginSession(ctx context.Context, id *loginID, els *emailLinkSignIn, confirmationCode string) error {
	sql := `UPDATE email_link_sign_ins
				   	  SET confirmation_code = $1
				WHERE email = $2
					  AND device_unique_id = $3
					  AND otp = $4
					  AND confirmation_code = $5`
	_, err := storage.Exec(ctx, c.db, sql, els.UserID, id.Email, id.DeviceUniqueID, els.OTP, confirmationCode)

	return errors.Wrapf(err, "failed to reset login session by id:%#v and confirmationCode:%v", id, confirmationCode)
}
