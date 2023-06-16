// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage/v2"
)

func (c *client) Status(ctx context.Context, email, deviceUniqueID string) (tokens *Tokens, err error) {
	loginSession := LoginSession(ctx)
	var token loginFlowToken
	if err = parseJwtToken(loginSession, c.cfg.LoginSession.JwtSecret, &token); err != nil {
		return nil, errors.Wrapf(err, "can't parse login flow token token:%v", loginSession)
	}
	id := ID{Email: email, DeviceUniqueID: deviceUniqueID}
	usr, err := c.getUserByLoginSession(ctx, loginSession, &id)
	if storage.IsErr(err, storage.ErrNotFound) {
		return nil, errors.Wrapf(ErrNoPendingLoginSession, "no pending login flow session:%v,id:%#v", loginSession, id)
	}
	if usr.ConfirmationCode != usr.UserID || usr.OTP != usr.UserID || usr.LoginSession == usr.UserID {
		return nil, errors.Wrapf(ErrStatusNotVerified, "not verified for id:%#v", id)
	}
	tokens, err = c.generateTokens(usr.TokenIssuedAt, usr, usr.IssuedTokenSeq)
	if err != nil {
		return nil, errors.Wrapf(err, "can't generate tokens for id:%#v", id)
	}
	if rErr := c.resetLoginSession(ctx, &id, loginSession); rErr != nil {
		return nil, errors.Wrapf(rErr, "can't reset login flow session:%v for id:%#v", loginSession, id)
	}

	return
}

func (c *client) resetLoginSession(ctx context.Context, id *ID, loginSession string) error {
	sql := `UPDATE email_link_sign_ins
				   	  SET login_session = email_link_sign_ins.user_id
				WHERE email = $1
					  AND device_unique_id = $2
					  AND login_session = $3`
	_, err := storage.Exec(ctx, c.db, sql, id.Email, id.DeviceUniqueID, loginSession)

	return errors.Wrapf(err, "failed to get user by loginSession:%v or id:%#v", loginSession, id)
}
