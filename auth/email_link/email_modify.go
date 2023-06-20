// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/email"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:funlen,revive // Big rollback logic.
func (c *client) handleEmailModification(ctx context.Context, els *emailLinkSignIns, newEmail, oldEmail, notifyEmail, confirmationCode string) error {
	usr := new(users.User)
	usr.ID = *els.UserID
	usr.Email = newEmail
	_, err := c.userModifier.ModifyUser(users.ConfirmedEmailContext(ctx, newEmail), usr, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to modify user %v with email modification", els.UserID)
	}
	if rErr := c.resetLoginSession(ctx, &loginID{Email: newEmail, DeviceUniqueID: els.DeviceUniqueID}, confirmationCode); rErr != nil {
		return multierror.Append( //nolint:wrapcheck // .
			errors.Wrapf(c.rollbackEmailModification(ctx, usr.ID, oldEmail), "[rollback] rollbackEmailModification failed for email:%v", oldEmail),
			errors.Wrapf(rErr, "failed to reset login session for email:%v", newEmail),
		).ErrorOrNil()
	}
	if notifyEmail != "" {
		rollbackEmailOTP, now := generateOTP(), time.Now()
		rollbackEmailPayload, rErr := c.generateMagicLinkPayload(&loginID{Email: oldEmail, DeviceUniqueID: els.DeviceUniqueID}, newEmail, "", rollbackEmailOTP, now)
		if rErr != nil {
			return multierror.Append( //nolint:wrapcheck // .
				errors.Wrapf(c.rollbackEmailModification(ctx, usr.ID, oldEmail), "[rollback] rollbackEmailModification failed for email:%v", oldEmail),
				errors.Wrapf(rErr, "can't generate link payload for email: %v", oldEmail),
			).ErrorOrNil()
		}
		rollbackConfirmationCode := generateConfirmationCode()
		if uErr := c.upsertEmailLinkSignIns(ctx, oldEmail, oldEmail, els.DeviceUniqueID, rollbackEmailOTP, rollbackConfirmationCode, now); uErr != nil {
			return multierror.Append( //nolint:wrapcheck // .
				errors.Wrapf(c.rollbackEmailModification(ctx, usr.ID, oldEmail), "[rollback] rollbackEmailModification failed for email:%v", oldEmail),
				errors.Wrapf(uErr, "failed to store/update email confirmation for email:%v", oldEmail),
			).ErrorOrNil()
		}
		authLink := c.getRollbackAuthLink(rollbackEmailPayload, els.Language, rollbackConfirmationCode)
		if sErr := c.sendNotifyEmailChanged(ctx, notifyEmail, authLink, els.Language); sErr != nil {
			return multierror.Append( //nolint:wrapcheck // .
				errors.Wrapf(c.rollbackEmailModification(ctx, usr.ID, oldEmail), "[rollback] rollbackEmailModification failed for email:%v", oldEmail),
				errors.Wrapf(sErr, "failed to send notification email about email change for userID %v email %v", els.UserID, oldEmail),
			).ErrorOrNil()
		}
	}

	return nil
}

func (c *client) getRollbackAuthLink(token, language, confirmationCode string) string {
	return fmt.Sprintf("%s?token=%s&lang=%s&confirmationCode=%s", c.cfg.EmailValidation.AuthLink, token, language, confirmationCode)
}

func (c *client) rollbackEmailModification(ctx context.Context, userID users.UserID, oldEmail string) error {
	usr := new(users.User)
	usr.ID = userID
	usr.Email = oldEmail
	_, err := c.userModifier.ModifyUser(users.ConfirmedEmailContext(ctx, oldEmail), usr, nil)

	return errors.Wrapf(err,
		"[rollback] failed to modify user:%v", userID)
}

func (c *client) sendNotifyEmailChanged(ctx context.Context, toEmail, link, language string) error {
	var tmpl *emailTemplate
	tmpl, ok := allEmailLinkTemplates[NotifyEmailChangedType][language]
	if !ok {
		tmpl = allEmailLinkTemplates[NotifyEmailChangedType][defaultLanguage]
	}
	data := struct {
		NewEmail string
		Link     string
	}{
		NewEmail: toEmail,
		Link:     link,
	}

	return errors.Wrapf(c.emailClient.Send(ctx, &email.Parcel{
		Body: &email.Body{
			Type: email.TextHTML,
			Data: tmpl.getBody(data),
		},
		Subject: tmpl.getSubject(nil),
		From: email.Participant{
			Name:  c.cfg.FromEmailName,
			Email: c.cfg.FromEmailAddress,
		},
	}, email.Participant{
		Name:  "",
		Email: toEmail,
	}), "failed to send notify email changed for user with email:%v", toEmail)
}
