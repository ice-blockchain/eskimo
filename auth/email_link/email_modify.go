// SPDX-License-Identifier: ice License 1.0

package emaillink

import (
	"bytes"
	"context"
	"text/template"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/email"
	"github.com/ice-blockchain/wintr/time"
)

func (r *repository) handleEmailModification(ctx context.Context, userID users.UserID, newEmail, oldEmail, notifyEmail string) error {
	usr := new(users.User)
	usr.ID = userID
	usr.Email = newEmail
	if err := r.userModifier.ModifyUser(users.ConfirmedEmailContext(ctx, newEmail), usr, nil); err != nil {
		return errors.Wrapf(err, "failed to modify user %v with email modification", userID)
	}
	if notifyEmail != "" {
		rollbackEmailOTP, now := generateOTP(), time.Now()
		rollbackEmailPayload, err := r.generateLinkPayload(oldEmail, newEmail, "", rollbackEmailOTP, now)
		if err != nil {
			return multierror.Append( //nolint:wrapcheck // .
				errors.Wrapf(r.rollbackEmailModification(ctx, userID, oldEmail), "[rollback] rollbackEmailModification failed for email:%v", oldEmail),
				errors.Wrapf(err, "can't generate link payload for email: %v", oldEmail),
			).ErrorOrNil()
		}
		if err = r.upsertEmailConfirmation(ctx, oldEmail, oldEmail, rollbackEmailOTP, now); err != nil {
			return multierror.Append( //nolint:wrapcheck // .
				errors.Wrapf(r.rollbackEmailModification(ctx, userID, oldEmail), "[rollback] rollbackEmailModification failed for email:%v", oldEmail),
				errors.Wrapf(err, "failed to store/update email confirmation for email:%v", oldEmail),
			).ErrorOrNil()
		}
		if err = r.sendNotifyEmailChanged(ctx, newEmail, notifyEmail, r.getAuthLink(rollbackEmailPayload)); err != nil {
			return multierror.Append( //nolint:wrapcheck // .
				errors.Wrapf(r.rollbackEmailModification(ctx, userID, oldEmail), "[rollback] rollbackEmailModification failed for email:%v", oldEmail),
				errors.Wrapf(err, "failed to send notification email about email change for userID %v email %v", userID, oldEmail),
			).ErrorOrNil()
		}
	}

	return nil
}

func (r *repository) rollbackEmailModification(ctx context.Context, userID users.UserID, oldEmail string) error {
	usr := new(users.User)
	usr.ID = userID
	usr.Email = oldEmail

	return errors.Wrapf(r.userModifier.ModifyUser(users.ConfirmedEmailContext(ctx, oldEmail), usr, nil),
		"[rollback] failed to modify user:%v", userID)
}

func (r *repository) sendNotifyEmailChanged(ctx context.Context, newEmail, toEmail, link string) error {
	emailTemplate, err := (new(template.Template).Parse(r.cfg.EmailValidation.NotifyChanged.EmailBodyHTMLTemplate))
	if err != nil {
		return errors.Wrapf(err, "invalid email template")
	}
	emailTemplateData := map[string]any{
		"newEmail":    newEmail,
		"link":        link,
		"serviceName": r.cfg.EmailValidation.ServiceName,
	}
	var emailMessageBuffer bytes.Buffer
	if eErr := emailTemplate.Execute(&emailMessageBuffer, emailTemplateData); eErr != nil {
		return errors.Wrapf(eErr,
			"invalid email template:%v or template data:%#v", r.cfg.EmailValidation.NotifyChanged.EmailBodyHTMLTemplate, emailTemplateData)
	}

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
