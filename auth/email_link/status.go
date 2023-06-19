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
		return nil, errors.Wrapf(err, "can't parse login flow token token:%v", loginSession)
	}
	id := ID{Email: token.Subject, DeviceUniqueID: token.DeviceUniqueID}
	usr, err := c.getConfirmedEmailLinkSignIns(ctx, &id, token.ConfirmationCode)
	if storage.IsErr(err, storage.ErrNotFound) {
		return nil, errors.Wrapf(ErrNoPendingLoginSession, "no pending login flow session:%v,id:%#v", loginSession, id)
	}
	if usr.ConfirmationCode == usr.UserID || usr.OTP != usr.UserID || !usr.Confirmed {
		return nil, errors.Wrapf(ErrStatusNotVerified, "not verified for id:%#v", id)
	}
	tokens, err = c.generateTokens(usr.TokenIssuedAt, usr, usr.IssuedTokenSeq)
	if err != nil {
		return nil, errors.Wrapf(err, "can't generate tokens for id:%#v", id)
	}
	if rErr := c.resetLoginSession(ctx, &id, token.ConfirmationCode); rErr != nil {
		return nil, errors.Wrapf(rErr, "can't reset login session for id:%#v", id)
	}

	return
}

func (c *client) resetLoginSession(ctx context.Context, id *ID, confirmationCode string) error {
	sql := `UPDATE email_link_sign_ins
				   	  SET confirmation_code = email_link_sign_ins.user_id
				WHERE email = $1
					  AND device_unique_id = $2
					  AND confirmation_code = $3`
	_, err := storage.Exec(ctx, c.db, sql, id.Email, id.DeviceUniqueID, confirmationCode)

	return errors.Wrapf(err, "failed to reset login session by id:%#v and confirmationCode:%v", id, confirmationCode)
}
