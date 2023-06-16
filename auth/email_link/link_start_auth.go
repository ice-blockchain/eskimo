// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/email"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

func (c *client) SendSignInLinkToEmail(ctx context.Context, emailValue, deviceUniqueID, language string) (loginSession, confirmationCode string, err error) {
	if ctx.Err() != nil {
		return "", "", errors.Wrap(ctx.Err(), "start email link auth failed because context failed")
	}
	id := ID{emailValue, deviceUniqueID}
	otp := generateOTP()
	now := time.Now()
	oldEmail := users.ConfirmedEmail(ctx)
	payload, err := c.generateMagicLinkPayload(&id, oldEmail, oldEmail, otp, now)
	if err != nil {
		return "", "", errors.Wrapf(err, "can't generate magic link payload for id: %#v", id)
	}
	confirmationCode = generateConfirmationCode()
	loginSession, err = c.generateLoginSession(&id, confirmationCode)
	if err != nil {
		return "", "", errors.Wrap(err, "can't call generateLoginSession")
	}
	if uErr := c.upsertEmailLinkSignIns(ctx, id.Email, oldEmail, id.DeviceUniqueID, otp, confirmationCode, now); uErr != nil {
		return "", "", errors.Wrapf(uErr, "failed to store/update email link sign ins for id:%#v", id)
	}
	authLink := c.getAuthLink(payload, language)
	if sErr := c.sendValidationEmail(ctx, id.Email, language, authLink); sErr != nil {
		return "", "", errors.Wrapf(sErr, "failed to send validation email for id:%#v", id)
	}

	return loginSession, confirmationCode, nil
}

func (c *client) sendValidationEmail(ctx context.Context, toEmail, language, link string) error {
	var tmpl *emailTemplate
	tmpl, ok := allEmailLinkTemplates[ValidationEmailType][language]
	if !ok {
		tmpl = allEmailLinkTemplates[ValidationEmailType][defaultLanguage]
	}
	data := struct {
		Email string
		Link  string
	}{
		Email: toEmail,
		Link:  link,
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
	}), "failed to send validation email for user with email:%v", toEmail)
}

//nolint:revive // .
func (c *client) upsertEmailLinkSignIns(ctx context.Context, toEmail, oldEmail, deviceUniqueID, otp, code string, now *time.Time) error {
	customClaimsFromOldEmail := "null"
	confirmationCodeWrongAttempts := 0
	confirmed := false
	params := []any{now.Time, toEmail, deviceUniqueID, otp, code, confirmationCodeWrongAttempts, confirmed}
	if oldEmail != "" {
		customClaimsFromOldEmail = "(SELECT custom_claims FROM email_link_sign_ins WHERE email = $8 AND device_unique_id = $3)"
		params = append(params, oldEmail)
	}
	sql := fmt.Sprintf(`INSERT INTO email_link_sign_ins (
							created_at,
							email,
							device_unique_id,
							otp,
							confirmation_code,
							confirmation_code_wrong_attempts_count,
							confirmation_code_created_at,
							confirmed,
							custom_claims)
						VALUES ($1, $2, $3, $4, $5, $6, $1, $7, %v)
						ON CONFLICT (email, device_unique_id) DO UPDATE 
							SET otp           				     	   = EXCLUDED.otp, 
								created_at    				     	   = EXCLUDED.created_at,
								confirmation_code 		          	   = EXCLUDED.confirmation_code,
								confirmation_code_created_at     	   = EXCLUDED.confirmation_code_created_at,
								confirmation_code_wrong_attempts_count = EXCLUDED.confirmation_code_wrong_attempts_count,
								confirmed 			     	   	   	   = EXCLUDED.confirmed,
								custom_claims 				     	   = EXCLUDED.custom_claims`, customClaimsFromOldEmail)
	_, err := storage.Exec(ctx, c.db, sql, params...)

	return errors.Wrapf(err, "failed to insert/update email link sign ins record for email:%v", toEmail)
}

func (c *client) generateMagicLinkPayload(id *ID, oldEmail, notifyEmail, otp string, now *time.Time) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, magicLinkToken{
		RegisteredClaims: &jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   id.Email,
			Audience:  nil,
			ExpiresAt: jwt.NewNumericDate(now.Add(c.cfg.EmailValidation.ExpirationTime)),
			NotBefore: jwt.NewNumericDate(*now.Time),
			IssuedAt:  jwt.NewNumericDate(*now.Time),
		},
		OTP:            otp,
		OldEmail:       oldEmail,
		NotifyEmail:    notifyEmail,
		DeviceUniqueID: id.DeviceUniqueID,
	})
	payload, err := token.SignedString([]byte(c.cfg.EmailValidation.JwtSecret))
	if err != nil {
		return "", errors.Wrapf(err, "can't generate link payload for id:%#v,otp:%v,now:%v", id, otp, now)
	}

	return payload, nil
}

func (c *client) getAuthLink(token, language string) string {
	return fmt.Sprintf("%s?token=%s&lang=%s", c.cfg.EmailValidation.AuthLink, token, language)
}

func (c *client) generateLoginSession(id *ID, confirmationCode string) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, loginFlowToken{
		RegisteredClaims: &jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   id.Email,
			Audience:  nil,
			ExpiresAt: jwt.NewNumericDate(now.Add(c.cfg.LoginSession.ExpirationTime)),
			NotBefore: jwt.NewNumericDate(*now.Time),
			IssuedAt:  jwt.NewNumericDate(*now.Time),
		},
		DeviceUniqueID:   id.DeviceUniqueID,
		ConfirmationCode: confirmationCode,
	})
	payload, err := token.SignedString([]byte(c.cfg.LoginSession.JwtSecret))
	if err != nil {
		return "", errors.Wrapf(err, "can't generate login flow for id:%#v,now:%v", id, now)
	}

	return payload, nil
}

func generateOTP() string {
	return uuid.NewString()
}

func generateConfirmationCode() string {
	result, err := rand.Int(rand.Reader, big.NewInt(999)) //nolint:gomnd // It's max value.
	log.Panic(err, "random wrong")

	return fmt.Sprintf("%03d", result.Int64()+1)
}
