// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage"
)

func (p *phoneNumberValidationCodes) generatePhoneValidationCode() string {
	return fmt.Sprintf("%04d", rand.Intn(9999-1)+1) //nolint:gomnd,gosec // Do we need super random here?
}

func (p *phoneNumberValidationCodes) sendValidationCode(number, code string) error {
	fmt.Println(number, code)
	//nolint:nolintlint    // TODO Here we send SMS to phone number.
	// If user specified a phoneNumber in the request body, then we proceed with phone number confirmation flow:
	// step 0: don`t update the phone number in users table
	// step 1: insert into phone_number_validation_codes // TODO ask Robert about the pattern of the code
	// step 2: use https://www.twilio.com/docs/libraries/go to send SMS with that code to the user`s phone number.

	return nil
}

func (p *phoneNumberValidationCodes) UpdatePhoneValidationCode(ctx context.Context, id UserID, number string) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "update phone number failed because context failed")
	}
	validationCode := p.generatePhoneValidationCode()
	if err := p.sendValidationCode(number, validationCode); err != nil {
		return errors.Wrapf(err, "failed to send validation code to phone number %v", number)
	}
	sql := fmt.Sprintf(`REPLACE INTO %v (ID, PHONE_NUMBER, VALIDATION_CODE, CREATED_AT) VALUES (:id, :phoneNumber, :validationCode, :createdAt)`, tableCodes)

	params := map[string]interface{}{
		"id":             id,
		"validationCode": validationCode,
		"phoneNumber":    number,
		"createdAt":      time.Now().UTC().UnixNano(),
	}

	query, err := p.db.PrepareExecute(sql, params)

	return errors.Wrapf(storage.CheckSQLDMLErr(query, err), "failed to update phone number %v", number)
}

func (p *phoneNumberValidationCodes) ConfirmPhoneNumber(ctx context.Context, conf *PhoneNumberConfirmation) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "check phone code failed because context failed")
	}

	result, err := p.getPhoneNumberValidationUser(ctx, conf.ID)
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

	if time.Since(time.Unix(int64(result.CreatedAt), 0)) > cfg.PhoneValidation.ExpirationTime {
		return ErrExpiredPhoneValidationCode
	}

	return nil
}

func (p *phoneNumberValidationCodes) getPhoneNumberValidationUser(_ context.Context, id UserID) (*phoneNumberValidationCode, error) {
	result := new(phoneNumberValidationCode)

	pk := fmt.Sprintf("pk_unnamed_%v_1", tableCodes)
	if err := p.db.GetTyped(tableCodes, pk, []interface{}{id}, result); err != nil {
		return &phoneNumberValidationCode{}, errors.Wrapf(err, "failed to get user by id %v", id)
	}

	return result, nil
}
