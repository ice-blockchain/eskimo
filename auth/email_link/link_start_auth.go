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

func (c *client) SendSignInLinkToEmail(ctx context.Context, emailValue, deviceUniqueID, language string) (loginSession string, err error) {
	if ctx.Err() != nil {
		return "", errors.Wrap(ctx.Err(), "send sign in link to email failed because context failed")
	}
	id := loginID{emailValue, deviceUniqueID}
	if vErr := c.validateEmailSignIn(ctx, &id); vErr != nil {
		return "", errors.Wrapf(vErr, "can't validate email sign in for:%#v", id)
	}
	oldEmail := users.ConfirmedEmail(ctx)
	if oldEmail != "" {
		oldID := loginID{oldEmail, deviceUniqueID}
		if vErr := c.validateEmailModification(ctx, emailValue, &oldID); vErr != nil {
			return "", errors.Wrapf(vErr, "can't validate modification email for:%#v", oldID)
		}
	}
	otp := generateOTP()
	confirmationCode := generateConfirmationCode()
	loginSession, err = c.generateLoginSession(&id, confirmationCode)
	if err != nil {
		return "", errors.Wrap(err, "can't call generateLoginSession")
	}
	now := time.Now()
	if uErr := c.upsertEmailLinkSignIn(ctx, id.Email, id.DeviceUniqueID, otp, confirmationCode, now); uErr != nil {
		return "", errors.Wrapf(uErr, "failed to store/update email link sign ins for id:%#v", id)
	}
	if sErr := c.sendMagicLink(ctx, &id, oldEmail, otp, language, now); sErr != nil {
		return "", errors.Wrapf(sErr, "can't send magic link for id:%#v", id)
	}

	return loginSession, nil
}

func (c *client) validateEmailSignIn(ctx context.Context, id *loginID) error {
	gUsr, err := c.getEmailLinkSignIn(ctx, id)
	if err != nil && !storage.IsErr(err, storage.ErrNotFound) {
		return errors.Wrapf(err, "can't get email link sign in information by:%#v", id)
	}
	if gUsr != nil && gUsr.BlockedUntil != nil {
		now := time.Now()
		if gUsr.BlockedUntil.After(*now.Time) {
			return errors.Wrapf(ErrUserBlocked, "user:%#v is blocked", id)
		}
	}

	return nil
}

func (c *client) validateEmailModification(ctx context.Context, newEmail string, oldID *loginID) error {
	if iErr := c.isUserExist(ctx, newEmail); !storage.IsErr(iErr, storage.ErrNotFound) {
		if iErr != nil {
			return errors.Wrapf(iErr, "can't check if user exists for email:%v", newEmail)
		}

		return errors.Wrapf(ErrUserDuplicate, "user with such email already exists:%v", newEmail)
	}
	gOldUsr, gErr := c.getEmailLinkSignIn(ctx, oldID)
	if gErr != nil && !storage.IsErr(gErr, storage.ErrNotFound) {
		return errors.Wrapf(gErr, "can't get email link sign in information by:%#v", oldID)
	}
	if gOldUsr != nil && gOldUsr.BlockedUntil != nil {
		now := time.Now()
		if gOldUsr.BlockedUntil.After(*now.Time) {
			return errors.Wrapf(ErrUserBlocked, "user:%#v is blocked", oldID)
		}
	}

	return nil
}

//nolint:revive // .
func (c *client) sendMagicLink(ctx context.Context, id *loginID, oldEmail, otp, language string, now *time.Time) error {
	payload, err := c.generateMagicLinkPayload(id, oldEmail, oldEmail, otp, now)
	if err != nil {
		return errors.Wrapf(err, "can't generate magic link payload for id: %#v", id)
	}
	authLink := c.getAuthLink(payload, language)
	var emailType string
	if oldEmail != "" {
		emailType = modifyEmailType
	} else {
		emailType = signInEmailType
	}

	return errors.Wrapf(c.sendEmailWithType(ctx, emailType, id.Email, language, authLink), "failed to send validation email for id:%#v", id)
}

func (c *client) sendEmailWithType(ctx context.Context, emailType, toEmail, language, link string) error {
	var tmpl *emailTemplate
	tmpl, ok := allEmailLinkTemplates[emailType][language]
	if !ok {
		tmpl = allEmailLinkTemplates[emailType][defaultLanguage]
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
	}), "failed to send email with type:%v for user with email:%v", emailType, toEmail)
}

//nolint:revive // .
func (c *client) upsertEmailLinkSignIn(ctx context.Context, toEmail, deviceUniqueID, otp, code string, now *time.Time) error {
	confirmationCodeWrongAttempts := 0
	params := []any{now.Time, toEmail, deviceUniqueID, otp, code, confirmationCodeWrongAttempts}
	sql := `INSERT INTO email_link_sign_ins (
							created_at,
							email,
							device_unique_id,
							otp,
							confirmation_code,
							confirmation_code_wrong_attempts_count)
						VALUES ($1, $2, $3, $4, $5, $6)
						ON CONFLICT (email, device_unique_id) DO UPDATE 
							SET otp           				     	   = EXCLUDED.otp, 
								created_at    				     	   = EXCLUDED.created_at,
								confirmation_code 		          	   = EXCLUDED.confirmation_code,
								confirmation_code_wrong_attempts_count = EXCLUDED.confirmation_code_wrong_attempts_count,
						        email_confirmed_at                     = null,
						        user_id                                = null
						WHERE   email_link_sign_ins.otp                                    != EXCLUDED.otp
						   OR   email_link_sign_ins.created_at    				     	   != EXCLUDED.created_at
						   OR   email_link_sign_ins.confirmation_code 		          	   != EXCLUDED.confirmation_code
						   OR   email_link_sign_ins.confirmation_code_wrong_attempts_count != EXCLUDED.confirmation_code_wrong_attempts_count`
	_, err := storage.Exec(ctx, c.db, sql, params...)

	return errors.Wrapf(err, "failed to insert/update email link sign ins record for email:%v", toEmail)
}

func (c *client) generateMagicLinkPayload(id *loginID, oldEmail, notifyEmail, otp string, now *time.Time) (string, error) {
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

func (c *client) generateLoginSession(id *loginID, confirmationCode string) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, loginFlowToken{
		RegisteredClaims: &jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   id.Email,
			Audience:  nil,
			ExpiresAt: jwt.NewNumericDate(now.Add(c.cfg.EmailValidation.ExpirationTime)),
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
