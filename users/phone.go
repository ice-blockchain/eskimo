// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/pkg/errors"

	"github.com/ICE-Blockchain/wintr/connectors/storage"
)

func (u *users) AddPhoneValidationCode(ctx context.Context, id UserID, number string) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "add phone number failed because context failed")
	}

	validationCode := u.generatePhoneValidationCode()

	if err := u.sendValidationCode(number, validationCode); err != nil {
		return errors.Wrapf(err, "failed to send validation code to phone number %v", number)
	}

	sql := fmt.Sprintf(`INSERT INTO %v (ID, PHONE_NUMBER, VALIDATION_CODE, CREATED_AT) VALUES (:id, :phoneNumber, :validationCode, :createdAt)`, tableCodes)

	params := map[string]interface{}{
		"id":             id,
		"phoneNumber":    number,
		"validationCode": validationCode,
		"createdAt":      time.Now().UTC().UnixNano(),
	}

	query, err := u.db.PrepareExecute(sql, params)
	if err = storage.CheckSQLDMLErr(query, err); err != nil {
		return errors.Wrapf(err, "failed to add phone number %#v", number)
	}

	// Do we send messages to mb?
	return nil
}

func (u *users) generatePhoneValidationCode() string {
	return fmt.Sprintf("%04d", rand.Intn(9999-1)+1) //nolint:gomnd,gosec // Do we need super random here?
}

func (u *users) sendValidationCode(number, code string) error {
	fmt.Println(number, code)
	//nolint:nolintlint    // TODO Here we send SMS to phone number.
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

	sql := fmt.Sprintf(`UPDATE %v SET validation_code = :validationCode, phone_number = :phoneNumber WHERE id = :userID`, tableCodes)

	params := map[string]interface{}{
		"userID":         id,
		"validationCode": validationCode,
		"phoneNumber":    number,
	}

	query, err := u.db.PrepareExecute(sql, params)
	if err = storage.CheckSQLDMLErr(query, err); err != nil {
		return errors.Wrapf(err, "failed to update phone number %v", number)
	}

	// Do we send messages to mb?
	return nil
}

func (u *users) PhoneNumberConfirmation(ctx context.Context, number, code string) (bool, error) {
	if ctx.Err() != nil {
		return false, errors.Wrap(ctx.Err(), "check phone code failed because context failed")
	}
	result := new(phoneNumberValidationCode)

	if err := u.db.GetTyped(tableCodes, "unique_unnamed_PHONE_NUMBER_VALIDATION_CODES_2", []interface{}{number}, result); err != nil {
		return false, errors.Wrapf(err, "failed to get user by phone number %v", number)
	}

	if result.ID == "" {
		return false, errors.Wrapf(ErrNotFound, "no user found with phone number %v", number)
	}

	if result.ValidationCode == code {
		err := u.updateUserPhone(ctx, result.PhoneNumber, result.ID)
		if err != nil {
			return false, errors.Wrapf(err, "error updating users")
		}

		return true, nil
	}

	return false, nil
}
