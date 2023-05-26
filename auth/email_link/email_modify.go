// SPDX-License-Identifier: ice License 1.0

package emaillink

import (
	"bytes"
	"context"
	"text/template"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/email"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

func (r *repository) handleEmailModification(ctx context.Context, userID users.UserID, newEmail, oldEmail, notifyEmail string) error {
	now := time.Now()
	rollbackOTP := generateOTP()
	rollbackToken, err := r.generateLinkPayload(oldEmail, newEmail, "", rollbackOTP, now)
	if err != nil {
		return errors.Wrapf(err, "can't generate link payload for email: %v", oldEmail)
	}
	if err := r.upsertPendingEmailConfirmation(ctx, oldEmail, oldEmail, rollbackOTP, now); err != nil {
		return errors.Wrap(err, "failed to store/update email confirmation")
	}
	usr := new(users.User)
	usr.ID = userID
	usr.Email = newEmail
	if err = r.userModifier.ModifyUser(users.ConfirmedEmailContext(ctx, newEmail), usr, nil); err != nil {
		return errors.Wrapf(err,
			"failed to modify user %v with email modification", userID)
	}
	if notifyEmail != "" {
		if err = r.sendNotifyEmailChanged(ctx, newEmail, notifyEmail, r.getAuthLink(rollbackToken)); err != nil {
			return errors.Wrapf(err, "failed to send notification email about email change for userID %v email %v", userID, oldEmail)
		}
	}
	return nil
}

func (r *repository) rollbackEmailModification(ctx context.Context, userID users.UserID, oldEmail string) error {
	usr := new(users.User)
	usr.ID = userID
	usr.Email = oldEmail

	return errors.Wrapf(r.userModifier.ModifyUser(users.ConfirmedEmailContext(ctx, oldEmail), usr, nil),
		"[rollback] failed to modify user %v", userID)
}

func (r *repository) sendNotifyEmailChanged(ctx context.Context, newEmail, toEmail, rollbackLink string) error {
	emailTemplate := template.Must(new(template.Template).Parse(r.cfg.EmailValidation.NotifyChanged.EmailBodyHTMLTemplate))
	emailTemplateData := map[string]any{
		"newEmail":    newEmail,
		"link":        rollbackLink,
		"serviceName": r.cfg.EmailValidation.ServiceName,
	}
	var emailMessageBuffer bytes.Buffer
	eErr := emailTemplate.Execute(&emailMessageBuffer, emailTemplateData)
	log.Panic(errors.Wrapf(eErr, "invalid Email template"))

	return errors.Wrapf(r.emailClient.Send(ctx, &email.Parcel{
		Body: &email.Body{
			Type: email.TextHTML,
			Data: emailMessageBuffer.String(),
		},
		Subject: r.cfg.EmailValidation.NotifyChanged.EmailSubject,
		From: email.Participant{
			Name:  r.cfg.EmailValidation.FromEmailName,
			Email: r.cfg.EmailValidation.FromEmailAddress,
		},
	}, email.Participant{
		Name:  "",
		Email: toEmail,
	}), "failed to send validation email for user with email:%v", toEmail)
}
