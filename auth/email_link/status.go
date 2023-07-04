// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/log"
)

//nolint:funlen // Logs
func (c *client) Status(ctx context.Context, loginSession string) (tokens *Tokens, emailConfirmed bool, err error) {
	var token loginFlowToken
	log.Debug(fmt.Sprintf("[auth/getConfirmationStatus] loginPayload: %v", loginSession))
	if err = parseJwtToken(loginSession, c.cfg.LoginSession.JwtSecret, &token); err != nil {
		log.Debug(fmt.Sprintf("[auth/getConfirmationStatus] token invalid: %v", loginSession))

		return nil, false, errors.Wrapf(err, "can't parse login session:%v", loginSession)
	}
	id := loginID{Email: token.Subject, DeviceUniqueID: token.DeviceUniqueID}
	els, err := c.getConfirmedEmailLinkSignIn(ctx, &id, token.ConfirmationCode)
	if err != nil && storage.IsErr(err, storage.ErrNotFound) {
		log.Debug(fmt.Sprintf("[auth/getConfirmationStatus] not found els: %v %#v", token.Subject, id))

		return nil, false, errors.Wrapf(ErrNoPendingLoginSession, "no pending login session:%v,id:%#v", loginSession, id)
	}
	if els.UserID == nil || els.OTP != *els.UserID {
		log.Debug(fmt.Sprintf("[auth/getConfirmationStatus] not confirmed yet: %#v", *els))

		return nil, false, errors.Wrapf(ErrStatusNotVerified, "not verified for id:%#v", id)
	}
	if els.ConfirmationCode == *els.UserID {
		log.Debug(fmt.Sprintf("[auth/getConfirmationStatus] tokens already issued: %#v", *els))

		return nil, false, errors.Wrapf(ErrNoPendingLoginSession, "tokens already provided for id:%#v", id)
	}
	tokens, err = c.generateTokens(els.TokenIssuedAt, els, els.IssuedTokenSeq)
	if err != nil {
		log.Debug(fmt.Sprintf("[auth/getConfirmationStatus] failed to issue tokens: %#v %v", *els, err))

		return nil, false, errors.Wrapf(err, "can't generate tokens for id:%#v", id)
	}
	if rErr := c.resetLoginSession(ctx, &id, els, token.ConfirmationCode); rErr != nil {
		log.Debug(fmt.Sprintf("[auth/getConfirmationStatus] reset failed: %#v %v", *els, rErr))

		return nil, false, errors.Wrapf(rErr, "can't reset login session for id:%#v", id)
	}
	emailConfirmed = els.EmailConfirmedAt != nil
	log.Debug(fmt.Sprintf("[auth/getConfirmationStatus] ok: %#v %v", tokens, emailConfirmed))

	return //nolint:nakedret // .
}

func (c *client) resetLoginSession(ctx context.Context, id *loginID, els *emailLinkSignIn, prevConfirmationCode string) error {
	sql := `UPDATE email_link_sign_ins
				   	  SET confirmation_code = $1
				WHERE email = $2
					  AND device_unique_id = $3
					  AND otp = $4
					  AND confirmation_code = $5
					  AND issued_token_seq = $6`
	_, err := storage.Exec(ctx, c.db, sql, els.UserID, id.Email, id.DeviceUniqueID, els.OTP, prevConfirmationCode, els.IssuedTokenSeq)

	return errors.Wrapf(err, "failed to reset login session by id:%#v and confirmationCode:%v", id, prevConfirmationCode)
}
