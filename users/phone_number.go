// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"text/template"
	"time"

	"github.com/pkg/errors"
	"github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"

	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/log"
)

func (u *users) generatePhoneValidationCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(999999)) //nolint:gomnd // static value
	if err != nil {
		log.Error(errors.Wrapf(err, "crypto random generator failed"))
	}

	return fmt.Sprintf("%06d", n.Uint64())
}

func (u *users) generateSMSMessage(code string) (string, error) {
	var b bytes.Buffer
	tpl := template.Must(template.New("smsMessage").Parse(cfg.PhoneNumberValidation.SmsTemplate))

	err := tpl.Execute(&b, map[string]interface{}{
		"code":           code,
		"expirationTime": cfg.PhoneNumberValidation.ExpirationTime.Minutes(),
	})
	if err != nil {
		return "", errors.Wrapf(err, "invalid SMS template")
	}

	return b.String(), nil
}

func (u *users) sendValidationCodeSMS(number, code string) error {
	msg, err := u.generateSMSMessage(code)
	if err != nil {
		return errors.Wrapf(err, "unable to generate validation SMS")
	}

	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: cfg.PhoneNumberValidation.TwilioCredentials.User,
		Password: cfg.PhoneNumberValidation.TwilioCredentials.Password,
	})

	params := &openapi.CreateMessageParams{}
	params.SetTo(number)
	params.SetFrom(cfg.PhoneNumberValidation.FromPhoneNumber)
	params.SetBody(msg)

	_, err = client.ApiV2010.CreateMessage(params)
	if err != nil {
		return errors.Wrapf(err, "twilio error")
	}

	return nil
}

func (u *users) updatePhoneValidationCode(ctx context.Context, conf *PhoneNumberConfirmation) error {
	sql := fmt.Sprintf(`REPLACE INTO %v (USER_ID, PHONE_NUMBER, PHONE_NUMBER_HASH, VALIDATION_CODE, CREATED_AT) VALUES
                                               (:id,     :phoneNumber, :phoneNumberHash,  :validationCode, :createdAt)`, tableCodes)
	params := map[string]interface{}{
		"id":              conf.UserID,
		"phoneNumber":     conf.PhoneNumber,
		"phoneNumberHash": conf.PhoneNumberHash,
		"createdAt":       time.Now().UTC().UnixNano(),
	}
	needSms := false
	for ctx.Err() == nil {
		if conf.ValidationCode == "" {
			needSms = true
			conf.ValidationCode = u.generatePhoneValidationCode()
		}
		params["validationCode"] = conf.ValidationCode
		query, err := u.db.PrepareExecute(sql, params)
		err = storage.CheckSQLDMLErr(query, err)
		if err != nil && !errors.Is(err, ErrDuplicate) {
			return errors.Wrapf(err, "failed updating validation code")
		}
		if err == nil {
			break
		}
	}
	if !needSms {
		return nil
	}

	return errors.Wrapf(u.sendValidationCodeSMS(conf.PhoneNumber, conf.ValidationCode), "failed to send validation SMS")
}

func (u *users) ConfirmPhoneNumber(ctx context.Context, conf *PhoneNumberConfirmation) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "check phone code failed because context failed")
	}
	result, err := u.getPhoneNumberValidationUser(ctx, conf.UserID)

	switch {
	case err != nil:
		return errors.Wrapf(err, "failed to get user by id %v", conf.UserID)
	case result.ID == "":
		return errors.Wrapf(ErrNotFound, "no user found with id %v", conf.UserID)
	case result.PhoneNumber != conf.PhoneNumber:
		return errors.Wrapf(ErrNotFound, "no phone %v waiting for confirmation", conf.PhoneNumber)
	case result.ValidationCode != conf.ValidationCode:
		return ErrInvalidPhoneValidationCode
	case time.Since(time.Unix(int64(result.CreatedAt), 0)) > cfg.PhoneNumberValidation.ExpirationTime:
		return ErrExpiredPhoneValidationCode
	}

	return errors.Wrapf(u.updateUserWithValidatedPhoneNumber(ctx, conf), "error updating user phone info")
}

func (u *users) updateUserWithValidatedPhoneNumber(ctx context.Context, conf *PhoneNumberConfirmation) error {
	user := new(User)
	user.ID = conf.UserID
	user.confirmedPhoneNumber = conf.PhoneNumber
	user.PhoneNumberHash = conf.PhoneNumberHash
	if err := u.ModifyUser(ctx, user); err != nil {
		return errors.Wrapf(err, "error updating users")
	}
	confirm := new(PhoneNumberConfirmation)
	confirm.UserID = user.ID
	confirm.PhoneNumber = conf.PhoneNumber
	// Just update the hash provided with the phone number.
	confirm.PhoneNumberHash = conf.PhoneNumberHash
	// According to Fedor, we're deactivating used code this way, to keep unique values in database.
	confirm.ValidationCode = user.ID

	return errors.Wrapf(u.updatePhoneValidationCode(ctx, confirm), "error updating validation code")
}

func (u *users) getPhoneNumberValidationUser(_ context.Context, id UserID) (*phoneNumberValidationCode, error) {
	result := new(phoneNumberValidationCode)

	pk := fmt.Sprintf("pk_unnamed_%v_1", tableCodes)
	if err := u.db.GetTyped(tableCodes, pk, []interface{}{id}, result); err != nil {
		return &phoneNumberValidationCode{}, errors.Wrapf(err, "failed to get user by id %v", id)
	}

	return result, nil
}
