// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/email"
	"github.com/ice-blockchain/wintr/time"
)

func (c *client) handleEmailModification(ctx context.Context, userID, newEmail, deviceUniqueID, oldEmail, notifyEmail, language string) error {
	usr := new(users.User)
	usr.ID = userID
	usr.Email = newEmail
	_, err := c.userModifier.ModifyUser(users.ConfirmedEmailContext(ctx, newEmail), usr, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to modify user %v with email modification", userID)
	}
	if notifyEmail != "" {
		rollbackEmailOTP, now := generateOTP(), time.Now()
		rollbackEmailPayload, rErr := c.generateMagicLinkPayload(&ID{Email: oldEmail, DeviceUniqueID: deviceUniqueID}, newEmail, "", rollbackEmailOTP, now)
		if rErr != nil {
			return multierror.Append( //nolint:wrapcheck // .
				errors.Wrapf(c.rollbackEmailModification(ctx, userID, oldEmail), "[rollback] rollbackEmailModification failed for email:%v", oldEmail),
				errors.Wrapf(rErr, "can't generate link payload for email: %v", oldEmail),
			).ErrorOrNil()
		}
		rollbackSession, rErr := c.generateLoginSession(&ID{Email: newEmail, DeviceUniqueID: deviceUniqueID})
		if rErr != nil {
			return errors.Wrap(rErr, "can't call generateLoginSession")
		}
		rollbackConfirmationCode := generateConfirmationCode()
		if uErr := c.upsertEmailLinkSignIns(ctx, oldEmail, oldEmail, deviceUniqueID, rollbackSession, rollbackEmailOTP, language, rollbackConfirmationCode, now); uErr != nil {
			return multierror.Append( //nolint:wrapcheck // .
				errors.Wrapf(c.rollbackEmailModification(ctx, userID, oldEmail), "[rollback] rollbackEmailModification failed for email:%v", oldEmail),
				errors.Wrapf(uErr, "failed to store/update email confirmation for email:%v", oldEmail),
			).ErrorOrNil()
		}
		authLink := c.getAuthLink(rollbackEmailPayload, language)
		if sErr := c.sendNotifyEmailChanged(ctx, notifyEmail, authLink, language); sErr != nil {
			return multierror.Append( //nolint:wrapcheck // .
				errors.Wrapf(c.rollbackEmailModification(ctx, userID, oldEmail), "[rollback] rollbackEmailModification failed for email:%v", oldEmail),
				errors.Wrapf(sErr, "failed to send notification email about email change for userID %v email %v", userID, oldEmail),
			).ErrorOrNil()
		}
	}

	return nil
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
