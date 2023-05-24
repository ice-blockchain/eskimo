// SPDX-License-Identifier: ice License 1.0

package emaillink

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/email"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

func (r *repository) StartEmailLinkAuth(ctx context.Context, au *Auth) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "create user failed because context failed")
	}
	otp := uuid.NewString()
	now := time.Now()
	token, err := r.generateLinkPayload(au.Email, otp, now)
	if err != nil {
		return errors.Wrapf(err, "can't generate link payload for email: %v", au.Email)
	}
	if uErr := r.upsertPendingEmailConfirmation(ctx, au.Email, otp, now); uErr != nil {
		return errors.Wrap(uErr, "failed to store/update email confirmation")
	}

	return errors.Wrap(r.sendValidationEmail(ctx, au.Email, r.getAuthLink(token)), "failed to store/update email confirmation")
}

func (r *repository) sendValidationEmail(ctx context.Context, toEmail, link string) error {
	emailTemplate := template.Must(new(template.Template).Parse(r.cfg.EmailValidation.EmailBodyHTMLTemplate))
	emailTemplateData := map[string]any{
		"email":       toEmail,
		"link":        link,
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
		Subject: fmt.Sprintf("Sign in to %v", r.cfg.EmailValidation.ServiceName),
		From: email.Participant{
			Name:  r.cfg.EmailValidation.FromEmailName,
			Email: r.cfg.EmailValidation.FromEmailAddress,
		},
	}, email.Participant{
		Name:  "",
		Email: toEmail,
	}), "failed to send validation email for user with email:%v", toEmail)
}

func (r *repository) upsertPendingEmailConfirmation(ctx context.Context, toEmail, otp string, now *time.Time) error {
	sql := `INSERT INTO pending_email_confirmations (created_at, email, otp)
	          VALUES ($1, $2, $3)
	          ON CONFLICT (email)
	          DO UPDATE SET otp = 		 EXCLUDED.otp, 
			  			    created_at = EXCLUDED.created_at`
	_, err := storage.Exec(ctx, r.db, sql, now.Time, toEmail, otp)

	return errors.Wrapf(err, "failed to insert/update email confirmation record for email:%v", toEmail)
}

func (r *repository) generateLinkPayload(emailValue, otp string, now *time.Time) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, emailClaims{
		RegisteredClaims: &jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   emailValue,
			Audience:  nil,
			ExpiresAt: jwt.NewNumericDate(now.Add(r.cfg.ExpirationTime)),
			NotBefore: jwt.NewNumericDate(*now.Time),
			IssuedAt:  jwt.NewNumericDate(*now.Time),
		},
		OTP: otp,
	})

	payload, err := token.SignedString([]byte(r.cfg.JWTSecret))

	return payload, errors.Wrapf(err, "can't generate link payload for email:%v,otp:%v,now:%v", emailValue, otp, now)
}

func (r *repository) getAuthLink(token string) string {
	return fmt.Sprintf("%s?token=%s", r.cfg.EmailValidation.AuthLink, token)
}
