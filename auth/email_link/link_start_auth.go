// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	stdlibtime "time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/email"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/terror"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:funlen,gocognit,revive //.
func (c *client) SendSignInLinkToEmail(ctx context.Context, emailValue, deviceUniqueID, language, clientIP string) (loginSession string, err error) {
	if ctx.Err() != nil {
		return "", errors.Wrap(ctx.Err(), "send sign in link to email failed because context failed")
	}
	id := loginID{emailValue, deviceUniqueID}
	now := time.Now()
	loginSessionNumber := now.Time.Unix() / int64(sameIPCheckRate.Seconds())
	if vErr := c.validateEmailSignIn(ctx, &id); vErr != nil {
		return "", errors.Wrapf(vErr, "can't validate email sign in for:%#v", id)
	}
	oldEmail := users.ConfirmedEmail(ctx)
	if oldEmail != "" {
		loginSessionNumber = 0
		clientIP = "" //nolint:revive // .
		oldID := loginID{oldEmail, deviceUniqueID}
		if vErr := c.validateEmailModification(ctx, emailValue, &oldID); vErr != nil {
			return "", errors.Wrapf(vErr, "can't validate modification email for:%#v", oldID)
		}
	}
	otp := generateOTP()
	confirmationCode := generateConfirmationCode()
	loginSession, err = c.generateLoginSession(&id, confirmationCode, clientIP, loginSessionNumber)
	if err != nil {
		return "", errors.Wrap(err, "can't call generateLoginSession")
	}
	if loginSessionNumber > 0 && clientIP != "" {
		if ipErr := c.upsertIPLoginAttempt(ctx, &id, clientIP, loginSessionNumber); ipErr != nil {
			return "", errors.Wrapf(ipErr, "failed increment login attempts for IP:%v (session num %v)", clientIP, loginSessionNumber)
		}
	}
	if uErr := c.upsertEmailLinkSignIn(ctx, id.Email, id.DeviceUniqueID, otp, confirmationCode, now); uErr != nil {
		if errors.Is(uErr, ErrUserDuplicate) {
			oldLoginSession, oErr := c.restoreOldLoginSession(ctx, &id, clientIP, loginSessionNumber)
			if oErr != nil {
				return "", multierror.Append( //nolint:wrapcheck // .
					errors.Wrapf(oErr, "failed to calculate oldLoginSession"),
					errors.Wrapf(uErr, "failed to store/update email link sign ins for id:%#v", id),
				).ErrorOrNil()
			}

			return oldLoginSession, nil
		}

		return "", multierror.Append( //nolint:wrapcheck // .
			errors.Wrapf(c.decrementIPLoginAttempts(ctx, clientIP, loginSessionNumber), "[rollback] failed to rollback login attempts for ip"),
			errors.Wrapf(uErr, "failed to store/update email link sign ins for id:%#v", id),
		).ErrorOrNil()
	}
	payload, pErr := c.generateMagicLinkPayload(&id, oldEmail, oldEmail, otp, now)
	if pErr != nil {
		return "", multierror.Append( //nolint:wrapcheck // .
			errors.Wrapf(c.decrementIPLoginAttempts(ctx, clientIP, loginSessionNumber), "[rollback] failed to rollback login attempts for ip"),
			errors.Wrapf(pErr, "can't generate magic link payload for id: %#v", id),
		).ErrorOrNil()
	}
	if sErr := c.sendMagicLink(ctx, &id, oldEmail, payload, language); sErr != nil {
		return "", multierror.Append( //nolint:wrapcheck // .
			errors.Wrapf(c.decrementIPLoginAttempts(ctx, clientIP, loginSessionNumber), "[rollback] failed to rollback login attempts for ip"),
			errors.Wrapf(sErr, "can't send magic link for id:%#v", id),
		).ErrorOrNil()
	}

	return loginSession, nil
}

func (c *client) restoreOldLoginSession(ctx context.Context, id *loginID, clientIP string, loginSessionNumber int64) (string, error) {
	existingSignIn, dErr := c.getEmailLinkSignIn(ctx, id, true)
	if dErr != nil {
		return "", multierror.Append( //nolint:wrapcheck // .
			errors.Wrapf(c.decrementIPLoginAttempts(ctx, clientIP, loginSessionNumber), "[rollback] failed to rollback login attempts for ip"),
			errors.Wrapf(dErr, "can't get email link sign in information by:%#v", id),
		).ErrorOrNil()
	}
	oldLoginSession, dErr := c.generateLoginSession(id, existingSignIn.ConfirmationCode, clientIP, loginSessionNumber)
	if dErr != nil {
		return "", multierror.Append( //nolint:wrapcheck // .
			errors.Wrapf(c.decrementIPLoginAttempts(ctx, clientIP, loginSessionNumber), "[rollback] failed to rollback login attempts for ip"),
			errors.Wrap(dErr, "can't generate loginSession"),
		).ErrorOrNil()
	}

	return oldLoginSession, errors.Wrapf(c.decrementIPLoginAttempts(ctx, clientIP, loginSessionNumber),
		"failed to rollback login attempts for ip due to reuse of loginSession")
}

func (c *client) validateEmailSignIn(ctx context.Context, id *loginID) error {
	gUsr, err := c.getEmailLinkSignIn(ctx, id, false)
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
	}

	return nil
}

func (c *client) decrementIPLoginAttempts(ctx context.Context, ip string, loginSessionNumber int64) error {
	if ip != "" && loginSessionNumber > 0 {
		sql := `UPDATE sign_ins_per_ip SET
					login_attempts = GREATEST(sign_ins_per_ip.login_attempts - 1, 0)
				WHERE ip = $1 AND login_session_number = $2`
		_, err := storage.Exec(ctx, c.db, sql, ip, loginSessionNumber)

		return errors.Wrapf(err, "failed to decrease login attempts for ip %v lsn %v", ip, loginSessionNumber)
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
	gOldUsr, gErr := c.getEmailLinkSignIn(ctx, oldID, false)
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

//nolint:revive,lll // .
func (c *client) upsertEmailLinkSignIn(ctx context.Context, toEmail, deviceUniqueID, otp, code string, now *time.Time) error {
	confirmationCodeWrongAttempts := 0
	params := []any{now.Time, toEmail, deviceUniqueID, otp, code, confirmationCodeWrongAttempts, userIDForPhoneNumberToEmailMigration(ctx)}
	sql := fmt.Sprintf(`INSERT INTO email_link_sign_ins (
							created_at,
							email,
							device_unique_id,
							otp,
							confirmation_code,
							confirmation_code_wrong_attempts_count,
							phone_number_to_email_migration_user_id)
						VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7,''))
						ON CONFLICT (email, device_unique_id) DO UPDATE 
							SET otp           				     	   = EXCLUDED.otp, 
								created_at    				     	   = EXCLUDED.created_at,
								confirmation_code 		          	   = EXCLUDED.confirmation_code,
								confirmation_code_wrong_attempts_count = EXCLUDED.confirmation_code_wrong_attempts_count,
								phone_number_to_email_migration_user_id = COALESCE(NULLIF(EXCLUDED.phone_number_to_email_migration_user_id,''),email_link_sign_ins.phone_number_to_email_migration_user_id),
						        email_confirmed_at                     = null,
						        user_id                                = null
						WHERE   (extract(epoch from email_link_sign_ins.created_at)::bigint/%[1]v)  != (extract(epoch from EXCLUDED.created_at::timestamp)::bigint/%[1]v)
						   AND (email_link_sign_ins.otp                                             != EXCLUDED.otp
						   OR   email_link_sign_ins.confirmation_code 		          	            != EXCLUDED.confirmation_code
						   OR   email_link_sign_ins.confirmation_code_wrong_attempts_count          != EXCLUDED.confirmation_code_wrong_attempts_count)`,
		uint64(duplicatedSignInRequestsInLessThan/stdlibtime.Second))
	rowsInserted, err := storage.Exec(ctx, c.db, sql, params...)
	if rowsInserted == 0 && err == nil {
		err = errors.Wrapf(ErrUserDuplicate, "duplicated signIn request for email %v,device %v", toEmail, deviceUniqueID)
	}

	return errors.Wrapf(err, "failed to insert/update email link sign ins record for email:%v", toEmail)
}

func (c *client) upsertIPLoginAttempt(ctx context.Context, id *loginID, clientIP string, loginSessionNumber int64) error {
	sql := `INSERT INTO sign_ins_per_ip (ip, login_session_number, login_attempts)
					VALUES ($1, $2, 1)
	ON CONFLICT (login_session_number, ip) DO UPDATE
		SET login_attempts = sign_ins_per_ip.login_attempts + 1`
	_, err := storage.Exec(ctx, c.db, sql, clientIP, loginSessionNumber)
	if err != nil {
		if storage.IsErr(err, storage.ErrCheckFailed) {
			err = errors.Wrapf(ErrTooManyAttempts, "user %#v is blocked due to a lot of requests from IP %v", id, clientIP)

			return terror.New(err, map[string]any{"ip": clientIP})
		}

		return errors.Wrapf(err, "failed to increment login attempts from IP %v", clientIP)
	}

	return nil
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

func (c *client) generateLoginSession(id *loginID, confirmationCode, clientIP string, loginSessionNumber int64) (string, error) {
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
		DeviceUniqueID:     id.DeviceUniqueID,
		ConfirmationCode:   confirmationCode,
		LoginSessionNumber: loginSessionNumber,
		ClientIP:           clientIP,
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
