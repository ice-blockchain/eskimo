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

	return fmt.Sprintf("%04d", rand.Intn(999999-1)+1) //nolint:gomnd,gosec // We don't need cryptosecure random
}

func (u *users) sendValidationCode(number, code string) error {
	msg := fmt.Sprintf("ice: %v is your confirmation code. It expires in %v minutes. Don’t share this code with anyone.",
		code,
		cfg.PhoneNumberValidation.ExpirationTime.Minutes(),
	)

	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: cfg.PhoneNumberValidation.User,
		Password: cfg.PhoneNumberValidation.Password,
	})

	params := &openapi.CreateMessageParams{}
	params.SetTo(number)
	params.SetFrom(cfg.PhoneNumberValidation.PhoneNumber)
	params.SetBody(msg)

	_, err := client.ApiV2010.CreateMessage(params)
	if err != nil {
		return errors.Wrapf(err, "twilio error")
	}

	return nil
}

func (u *users) UpdatePhoneValidationCode(ctx context.Context, id UserID, number string) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "update phone number failed because context failed")
	}
	validationCode := u.generatePhoneValidationCode()
	if err := u.sendValidationCode(number, validationCode); err != nil {
		return errors.Wrapf(err, "failed to send validation code to phone number %v", number)
	}
	sql := fmt.Sprintf(`REPLACE INTO %v (X_ID, PHONE_NUMBER, VALIDATION_CODE, CREATED_AT) VALUES (:id, :phoneNumber, :validationCode, :createdAt)`, tableCodes)

	params := map[string]interface{}{
		"id":             id,
		"validationCode": validationCode,
		"phoneNumber":    number,
		"createdAt":      time.Now().UTC().UnixNano(),
	}

	query, err := u.db.PrepareExecute(sql, params)

	return errors.Wrapf(storage.CheckSQLDMLErr(query, err), "failed to update phone number %v", number)
}

func (u *users) ConfirmPhoneNumber(ctx context.Context, conf *PhoneNumberConfirmation) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "check phone code failed because context failed")
	}

	result, err := u.getPhoneNumberValidationUser(ctx, conf.ID)
	if err != nil {
		return errors.Wrapf(err, "failed to get user by id %v", conf.ID)
	}

	if result.ID == "" {
		return errors.Wrapf(ErrNotFound, "no user found with id %v", conf.ID)
	}

	if result.PhoneNumber != conf.PhoneNumber {
		return errors.Wrapf(ErrNotFound, "no phone %v waiting for confirmation", conf.PhoneNumber)
	}

	if result.ValidationCode != conf.ValidationCode {
		return ErrInvalidPhoneValidationCode
	}

	if time.Since(time.Unix(int64(result.CreatedAt), 0)) > cfg.PhoneNumberValidation.ExpirationTime {
		return ErrExpiredPhoneValidationCode
	}

	user := new(User)
	user.ID = conf.ID
	user.confirmedPhoneNumber = conf.PhoneNumber

	return errors.Wrapf(u.ModifyUser(ctx, user), "error updating users")
}

func (u *users) getPhoneNumberValidationUser(_ context.Context, id UserID) (*phoneNumberValidationCode, error) {
	result := new(phoneNumberValidationCode)

	pk := fmt.Sprintf("pk_unnamed_%v_1", tableCodes)
	if err := u.db.GetTyped(tableCodes, pk, []interface{}{id}, result); err != nil {
		return &phoneNumberValidationCode{}, errors.Wrapf(err, "failed to get user by id %v", id)
	}

	return result, nil
}
