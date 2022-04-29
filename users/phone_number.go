// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/pkg/errors"
	"github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"

	"github.com/ice-blockchain/wintr/connectors/storage"
)

func (u *users) generatePhoneValidationCode() string {
	rand.Seed(time.Now().UnixNano())

	return fmt.Sprintf("%06d", rand.Intn(999999-1)+1) //nolint:gomnd,gosec // We don't need cryptosecure random
}

func (u *users) sendValidationCodeSMS(number, code string) error {
	msg := fmt.Sprintf("%v is your ice confirmation code. It expires in %v minutes. Don’t share this code with anyone.",
		code,
		cfg.PhoneNumberValidation.ExpirationTime.Minutes(),
	)

	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: cfg.PhoneNumberValidation.TwilioCredentials.User,
		Password: cfg.PhoneNumberValidation.TwilioCredentials.Password,
	})

	params := &openapi.CreateMessageParams{}
	params.SetTo(number)
	params.SetFrom(cfg.PhoneNumberValidation.FromPhoneNumber)
	params.SetBody(msg)

	_, err := client.ApiV2010.CreateMessage(params)
	if err != nil {
		return errors.Wrapf(err, "twilio error")
	}

	return nil
}

func (u *users) updatePhoneValidationCode(ctx context.Context, conf *PhoneNumberConfirmation) error {
	sql := fmt.Sprintf(`REPLACE INTO %v (USER_ID, PHONE_NUMBER, VALIDATION_CODE, CREATED_AT) VALUES (:id, :phoneNumber, :validationCode, :createdAt)`, tableCodes)
	params := map[string]interface{}{
		"id":          conf.ID,
		"phoneNumber": conf.PhoneNumber,
		"createdAt":   time.Now().UTC().UnixNano(),
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
	result, err := u.getPhoneNumberValidationUser(ctx, conf.ID)

	switch {
	case err != nil:
		return errors.Wrapf(err, "failed to get user by id %v", conf.ID)
	case result.ID == "":
		return errors.Wrapf(ErrNotFound, "no user found with id %v", conf.ID)
	case result.PhoneNumber != conf.PhoneNumber:
		return errors.Wrapf(ErrNotFound, "no phone %v waiting for confirmation", conf.PhoneNumber)
	case result.ValidationCode != conf.ValidationCode:
		return ErrInvalidPhoneValidationCode
	case time.Since(time.Unix(int64(result.CreatedAt), 0)) > cfg.PhoneNumberValidation.ExpirationTime:
		return ErrExpiredPhoneValidationCode
	}

	user := new(User)
	user.ID = conf.ID
	user.confirmedPhoneNumber = conf.PhoneNumber
	if err = u.ModifyUser(ctx, user); err != nil {
		return errors.Wrapf(err, "error updating users")
	}
	confirm := new(PhoneNumberConfirmation)
	confirm.ID = user.ID
	confirm.PhoneNumber = conf.PhoneNumber
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
