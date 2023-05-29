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

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/auth"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/email"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

func (r *repository) StartEmailLinkAuth(ctx context.Context, emailValue string) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "start email link auth failed because context failed")
	}
	otp := generateOTP()
	now := time.Now()
	oldEmail := users.ConfirmedEmail(ctx)
	token, err := r.generateLinkPayload(emailValue, oldEmail, oldEmail, otp, now)
	if err != nil {
		return errors.Wrapf(err, "can't generate link payload for email: %v", emailValue)
	}
	if uErr := r.upsertPendingEmailConfirmation(ctx, emailValue, oldEmail, otp, now); uErr != nil {
		return errors.Wrapf(uErr, "failed to store/update email confirmation for:%v", emailValue)
	}

	return errors.Wrapf(r.sendValidationEmail(ctx, emailValue, r.getAuthLink(token)), "failed to send validation email for:%v", emailValue)
}

func (r *repository) sendValidationEmail(ctx context.Context, toEmail, link string) error {
	emailTemplate := template.Must(new(template.Template).Parse(r.cfg.EmailValidation.SignIn.EmailBodyHTMLTemplate))
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
		Subject: fmt.Sprintf("Verify your email for %v", r.cfg.EmailValidation.ServiceName),
		From: email.Participant{
			Name:  r.cfg.EmailValidation.FromEmailName,
			Email: r.cfg.EmailValidation.FromEmailAddress,
		},
	}, email.Participant{
		Name:  "",
		Email: toEmail,
	}), "failed to send validation email for user with email:%v", toEmail)
}

func (r *repository) upsertPendingEmailConfirmation(ctx context.Context, toEmail, oldEmail, otp string, now *time.Time) error {
	customClaimsFromOldEmail := "null"
	params := []any{now.Time, toEmail, otp}
	if oldEmail != "" {
		customClaimsFromOldEmail = "(SELECT custom_claims FROM pending_email_confirmations WHERE email = $4)"
		params = append(params, oldEmail)
	}
	sql := fmt.Sprintf(`INSERT INTO pending_email_confirmations (created_at, email, otp, custom_claims)
	          VALUES ($1, $2, $3, %v)
	          ON CONFLICT (email)
	          DO UPDATE SET otp           = EXCLUDED.otp, 
			  			    created_at    = EXCLUDED.created_at,
	                        custom_claims = EXCLUDED.custom_claims`, customClaimsFromOldEmail)
	_, err := storage.Exec(ctx, r.db, sql, params...)

	return errors.Wrapf(err, "failed to insert/update email confirmation record for email:%v", toEmail)
}

func (r *repository) generateLinkPayload(emailValue, oldEmail, notifyEmail, otp string, now *time.Time) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, emailClaims{
		RegisteredClaims: &jwt.RegisteredClaims{
			Issuer:    auth.JwtIssuer,
			Subject:   emailValue,
			Audience:  nil,
			ExpiresAt: jwt.NewNumericDate(now.Add(r.cfg.EmailExpirationTime)),
			NotBefore: jwt.NewNumericDate(*now.Time),
			IssuedAt:  jwt.NewNumericDate(*now.Time),
		},
		OTP:         otp,
		OldEmail:    oldEmail,
		NotifyEmail: notifyEmail,
	})
	payload, err := token.SignedString([]byte(r.cfg.EmailJWTSecret))

	return payload, errors.Wrapf(err, "can't generate link payload for email:%v,otp:%v,now:%v", emailValue, otp, now)
}

func (r *repository) getAuthLink(token string) string {
	return fmt.Sprintf("%s?token=%s", r.cfg.EmailValidation.AuthLink, token)
}

func generateOTP() string {
	return uuid.NewString()
}
