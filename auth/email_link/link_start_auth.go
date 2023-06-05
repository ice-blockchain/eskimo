// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/email"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

func (c *client) SendSignInLinkToEmail(ctx context.Context, email string) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "start email link auth failed because context failed")
	}
	otp := generateOTP()
	now := time.Now()
	oldEmail := users.ConfirmedEmail(ctx)
	token, err := c.generateLinkPayload(email, oldEmail, oldEmail, otp, now)
	if err != nil {
		return errors.Wrapf(err, "can't generate link payload for email: %v", email)
	}
	if uErr := c.upsertEmailConfirmation(ctx, email, oldEmail, otp, now); uErr != nil {
		return errors.Wrapf(uErr, "failed to store/update email confirmation for:%v", email)
	}

	return errors.Wrapf(c.sendValidationEmail(ctx, email, c.getAuthLink(token)), "failed to send validation email for:%v", email)
}

func (c *client) sendValidationEmail(ctx context.Context, toEmail, link string) error {
	emailTemplate := template.Must(new(template.Template).Parse(c.cfg.EmailValidation.SignIn.EmailBodyHTMLTemplate))
	emailTemplateData := map[string]any{
		"email": toEmail,
		"link":  link,
	}
	var emailMessageBuffer bytes.Buffer
	eErr := emailTemplate.Execute(&emailMessageBuffer, emailTemplateData)
	log.Panic(errors.Wrapf(eErr, "invalid Email template"))

	return errors.Wrapf(c.emailClient.Send(ctx, &email.Parcel{
		Body: &email.Body{
			Type: email.TextHTML,
			Data: emailMessageBuffer.String(),
		},
		Subject: c.cfg.EmailValidation.SignIn.EmailSubject,
		From: email.Participant{
			Name:  c.cfg.EmailValidation.FromEmailName,
			Email: c.cfg.EmailValidation.FromEmailAddress,
		},
	}, email.Participant{
		Name:  "",
		Email: toEmail,
	}), "failed to send validation email for user with email:%v", toEmail)
}

func (c *client) upsertEmailConfirmation(ctx context.Context, toEmail, oldEmail, otp string, now *time.Time) error {
	customClaimsFromOldEmail := "null"
	params := []any{now.Time, toEmail, otp}
	if oldEmail != "" {
		customClaimsFromOldEmail = "(SELECT custom_claims FROM email_link_sign_ins WHERE email = $4)"
		params = append(params, oldEmail)
	}
	sql := fmt.Sprintf(`INSERT INTO email_link_sign_ins (created_at, email, otp, custom_claims)
						VALUES ($1, $2, $3, %v)
						ON CONFLICT (email)
						DO UPDATE SET otp           = EXCLUDED.otp, 
			  			              created_at    = EXCLUDED.created_at,
									  custom_claims = EXCLUDED.custom_claims`, customClaimsFromOldEmail)
	_, err := storage.Exec(ctx, c.db, sql, params...)

	return errors.Wrapf(err, "failed to insert/update email confirmation record for email:%v", toEmail)
}

func (c *client) generateLinkPayload(emailValue, oldEmail, notifyEmail, otp string, now *time.Time) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, emailClaims{
		RegisteredClaims: &jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   emailValue,
			Audience:  nil,
			ExpiresAt: jwt.NewNumericDate(now.Add(c.cfg.EmailExpirationTime)),
			NotBefore: jwt.NewNumericDate(*now.Time),
			IssuedAt:  jwt.NewNumericDate(*now.Time),
		},
		OTP:         otp,
		OldEmail:    oldEmail,
		NotifyEmail: notifyEmail,
	})
	payload, err := token.SignedString([]byte(c.cfg.EmailJWTSecret))

	return payload, errors.Wrapf(err, "can't generate link payload for email:%v,otp:%v,now:%v", emailValue, otp, now)
}

func (c *client) getAuthLink(token string) string {
	return fmt.Sprintf("%s?token=%s", c.cfg.EmailValidation.AuthLink, token)
}

func generateOTP() string {
	return uuid.NewString()
}
