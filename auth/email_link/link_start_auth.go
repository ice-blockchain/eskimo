// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	stdlibtime "time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/email"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/terror"
	"github.com/ice-blockchain/wintr/time"
)

func (c *client) SendSignInLinkToEmail(ctx context.Context, emailValue, deviceUniqueID, language string, clientIP net.IP) (loginSession string, err error) {
	if ctx.Err() != nil {
		return "", errors.Wrap(ctx.Err(), "send sign in link to email failed because context failed")
	}
	id := loginID{emailValue, deviceUniqueID}
	now := time.Now()
	loginSessionNumber := int64(0)
	if c.cfg.EmailValidation.SameIPRateCheckPeriod.Seconds() > 0 && c.cfg.EmailValidation.MaxRequestsFromIP > 0 {
		loginSessionNumber = now.Time.Unix() / int64(c.cfg.EmailValidation.SameIPRateCheckPeriod.Seconds())
	}
	if vErr := c.validateEmailSignIn(ctx, &id, clientIP, loginSessionNumber); vErr != nil {
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
	if uErr := c.upsertEmailLinkSignIn(ctx, id.Email, id.DeviceUniqueID, otp, confirmationCode, now, clientIP, loginSessionNumber); uErr != nil {
		return "", errors.Wrapf(uErr, "failed to store/update email link sign ins for id:%#v", id)
	}
	payload, pErr := c.generateMagicLinkPayload(&id, oldEmail, oldEmail, otp, now, loginSessionNumber, clientIP)
	if pErr != nil {
		return "", errors.Wrapf(pErr, "can't generate magic link payload for id: %#v", id)
	}
	if sErr := c.sendMagicLink(ctx, &id, oldEmail, payload, language); sErr != nil {
		return "", errors.Wrapf(sErr, "can't send magic link for id:%#v", id)
	}

	return loginSession, nil
}

func (c *client) validateEmailSignIn(ctx context.Context, id *loginID, clientIP net.IP, loginSessionNumber int64) error {
	gUsr, err := c.getEmailLinkSignIn(ctx, id)
	if err != nil && !storage.IsErr(err, storage.ErrNotFound) {
		return errors.Wrapf(err, "can't get email link sign in information by:%#v", id)
	}
	now := time.Now()
	if gUsr != nil {
		if gUsr.BlockedUntil != nil {
			if gUsr.BlockedUntil.After(*now.Time) {
				err = errors.Wrapf(ErrUserBlocked, "user:%#v is blocked due to a lot of incorrect codes", id)
				return terror.New(err, map[string]any{"source": "email"})
			}
		}
		if c.cfg.EmailValidation.MaxRequestsFromIP > 0 &&
			gUsr.LoginAttempts >= c.cfg.EmailValidation.MaxRequestsFromIP &&
			loginSessionNumber == gUsr.LoginSessionNumber &&
			clientIP.String() == gUsr.IP {
			err = errors.Wrapf(ErrUserBlocked, "user %#v is blocked due to a lot of requests from IP %v", id, clientIP.String())
			return terror.New(err, map[string]any{"source": "ip"})
		}
	}

	return nil
}

func (c *client) validateEmailModification(ctx context.Context, newEmail string, oldID *loginID) error {
	if iErr := c.isUserExist(ctx, newEmail); !storage.IsErr(iErr, storage.ErrNotFound) {
		if iErr != nil {
			return errors.Wrapf(iErr, "can't check if user exists for email:%v", newEmail)
		}

		return errors.Wrapf(terror.New(ErrUserDuplicate, map[string]any{"field": "email"}), "user with such email already exists:%v", newEmail)
	}
	gOldUsr, gErr := c.getEmailLinkSignIn(ctx, oldID)
	if gErr != nil && !storage.IsErr(gErr, storage.ErrNotFound) {
		return errors.Wrapf(gErr, "can't get email link sign in information by:%#v", oldID)
	}
	if gOldUsr != nil && gOldUsr.BlockedUntil != nil {
		now := time.Now()
		if gOldUsr.BlockedUntil.After(*now.Time) {
			err := errors.Wrapf(ErrUserBlocked, "user:%#v is blocked", oldID)
			return terror.New(err, map[string]any{"source": "email"})
		}
	}

	return nil
}

//nolint:revive // .
func (c *client) sendMagicLink(ctx context.Context, id *loginID, oldEmail, payload, language string) error {
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

//nolint:revive, funlen // .
func (c *client) upsertEmailLinkSignIn(ctx context.Context, toEmail, deviceUniqueID, otp, code string, now *time.Time, clientIP net.IP, loginSessionNumber int64) error {
	params := []any{now.Time, toEmail, deviceUniqueID, otp, code, clientIP.String(), loginSessionNumber}
	ipBlockEndTime := stdlibtime.Unix(loginSessionNumber*int64(c.cfg.EmailValidation.SameIPRateCheckPeriod.Seconds()), 0).
		Add(c.cfg.EmailValidation.SameIPRateCheckPeriod)
	params = append(params, ipBlockEndTime)
	maxRequestsExceededCondition := fmt.Sprintf("sign_ins_per_ip.login_attempts > %[1]v", c.cfg.EmailValidation.MaxRequestsFromIP)

	type loginAttempt struct {
		LoginAttempts int64
	}
	sql := fmt.Sprintf(`
WITH ip_update AS (
	INSERT INTO sign_ins_per_ip (ip, login_session_number, login_attempts)
					VALUES ($6, $7, 1)
	ON CONFLICT (login_session_number, ip) DO UPDATE
		SET login_attempts = sign_ins_per_ip.login_attempts + 1,
			blocked_until = (CASE WHEN %[1]v THEN $8::timestamp ELSE null END)
	RETURNING (CASE WHEN %[1]v THEN (select -1 as login_attempts from generate_series(0, -1) x)
		ELSE (sign_ins_per_ip.login_attempts) END)
)
INSERT INTO email_link_sign_ins (
							created_at,
							email,
							device_unique_id,
							otp,
							confirmation_code,
							confirmation_code_wrong_attempts_count,
                            ip,
                            login_session_number,
							login_attempts
                            )
						VALUES ($1::timestamp, $2, $3, $4, $5, 0, $6, $7, COALESCE((SELECT login_attempts FROM ip_update), %[2]v))
						ON CONFLICT (email, device_unique_id) DO UPDATE 
							SET otp           				     	   = EXCLUDED.otp, 
								created_at    				     	   = EXCLUDED.created_at,
								confirmation_code 		          	   = EXCLUDED.confirmation_code,
								confirmation_code_wrong_attempts_count = EXCLUDED.confirmation_code_wrong_attempts_count,
						        email_confirmed_at                     = null,
						        user_id                                = null,
						        ip                                     = EXCLUDED.ip,
						        login_session_number                   = EXCLUDED.login_session_number,
								login_attempts                         = EXCLUDED.login_attempts
						WHERE   email_link_sign_ins.otp                                    != EXCLUDED.otp
						   OR   email_link_sign_ins.created_at    				     	   != EXCLUDED.created_at
						   OR   email_link_sign_ins.confirmation_code 		          	   != EXCLUDED.confirmation_code
						   OR   email_link_sign_ins.confirmation_code_wrong_attempts_count != EXCLUDED.confirmation_code_wrong_attempts_count
						   OR   email_link_sign_ins.ip 									   != EXCLUDED.ip
						   OR   email_link_sign_ins.login_session_number 				   != EXCLUDED.login_session_number
						RETURNING email_link_sign_ins.login_attempts
				`, maxRequestsExceededCondition, c.cfg.EmailValidation.MaxRequestsFromIP+1)
	attempts, err := storage.ExecOne[loginAttempt](ctx, c.db, sql, params...)
	if err != nil {
		if !storage.IsErr(err, storage.ErrNotFound) {
			return errors.Wrapf(err, "failed to insert/update email link sign ins record for email:%v", toEmail)
		}
		attempts.LoginAttempts = c.cfg.EmailValidation.MaxRequestsFromIP + 1
	}
	if c.cfg.EmailValidation.MaxRequestsFromIP > 0 && attempts.LoginAttempts > c.cfg.EmailValidation.MaxRequestsFromIP {
		err = errors.Wrapf(ErrUserBlocked, "email %v (device %v) is blocked due to a lot of requests from IP %v", toEmail, deviceUniqueID, clientIP.String())

		return terror.New(err, map[string]any{"source": "ip"})
	}

	return nil
}

func (c *client) generateMagicLinkPayload(id *loginID, oldEmail, notifyEmail, otp string, now *time.Time, loginSessionNumber int64, clientIP net.IP) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, magicLinkToken{
		RegisteredClaims: &jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   id.Email,
			Audience:  nil,
			ExpiresAt: jwt.NewNumericDate(now.Add(c.cfg.EmailValidation.ExpirationTime)),
			NotBefore: jwt.NewNumericDate(*now.Time),
			IssuedAt:  jwt.NewNumericDate(*now.Time),
		},
		OTP:                otp,
		OldEmail:           oldEmail,
		NotifyEmail:        notifyEmail,
		DeviceUniqueID:     id.DeviceUniqueID,
		ClientIP:           clientIP.String(),
		LoginSessionNumber: loginSessionNumber,
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
