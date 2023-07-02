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

//nolint:funlen // Big rollback logic.
func (c *client) handleEmailModification(ctx context.Context, els *emailLinkSignIn, newEmail, oldEmail, notifyEmail string) error {
	usr := new(users.User)
	usr.ID = *els.UserID
	usr.Email = newEmail
	err := c.userModifier.ModifyUser(users.ConfirmedEmailContext(ctx, newEmail), usr, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to modify user %v with email modification", els.UserID)
	}
	if notifyEmail != "" {
		resetEmailOTP, now := generateOTP(), time.Now()
		resetEmailPayload, rErr := c.generateMagicLinkPayload(&loginID{Email: oldEmail, DeviceUniqueID: els.DeviceUniqueID}, newEmail, "", resetEmailOTP, now)
		if rErr != nil {
			return multierror.Append( //nolint:wrapcheck // .
				errors.Wrapf(c.resetEmailModification(ctx, usr.ID, oldEmail), "[reset] resetEmailModification failed for email:%v", oldEmail),
				errors.Wrapf(rErr, "can't generate link payload for email: %v", oldEmail),
			).ErrorOrNil()
		}
		resetConfirmationCode := generateConfirmationCode()
		if uErr := c.upsertEmailLinkSignIn(ctx, oldEmail, oldEmail, els.DeviceUniqueID, resetEmailOTP, resetConfirmationCode, now); uErr != nil {
			return multierror.Append( //nolint:wrapcheck // .
				errors.Wrapf(c.resetEmailModification(ctx, usr.ID, oldEmail), "[reset] resetEmailModification failed for email:%v", oldEmail),
				errors.Wrapf(uErr, "failed to store/update email confirmation for email:%v", oldEmail),
			).ErrorOrNil()
		}
		authLink := c.getResetAuthLink(resetEmailPayload, els.Language, resetConfirmationCode)
		if sErr := c.sendNotifyEmailChanged(ctx, notifyEmail, newEmail, authLink, els.Language); sErr != nil {
			return multierror.Append( //nolint:wrapcheck // .
				errors.Wrapf(c.resetEmailModification(ctx, usr.ID, oldEmail), "[reset] resetEmailModification failed for email:%v", oldEmail),
				errors.Wrapf(sErr, "failed to send notification email about email change for userID %v email %v", els.UserID, oldEmail),
			).ErrorOrNil()
		}
	}

	return nil
}

func (c *client) getResetAuthLink(token, language, confirmationCode string) string {
	return fmt.Sprintf("%s?token=%s&lang=%s&confirmationCode=%s", c.cfg.EmailValidation.AuthLink, token, language, confirmationCode)
}

func (c *client) resetEmailModification(ctx context.Context, userID users.UserID, oldEmail string) error {
	usr := new(users.User)
	usr.ID = userID
	usr.Email = oldEmail

	return errors.Wrapf(c.userModifier.ModifyUser(users.ConfirmedEmailContext(ctx, oldEmail), usr, nil),
		"[rollback] failed to modify user:%v", userID)
}

func (c *client) sendNotifyEmailChanged(ctx context.Context, notifyEmail, newEmail, link, language string) error {
	var tmpl *emailTemplate
	tmpl, ok := allEmailLinkTemplates[notifyEmailChangedType][language]
	if !ok {
		tmpl = allEmailLinkTemplates[notifyEmailChangedType][defaultLanguage]
	}
	data := struct {
		NewEmail string
		Link     string
	}{
		NewEmail: newEmail,
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
		Email: notifyEmail,
	}), "failed to send notify email changed for user with email:%v", notifyEmail)
}
