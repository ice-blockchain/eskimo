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

func (u *users) addPhoneValidationCode(ctx context.Context, id UserID, number string) error {
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

	return errors.Wrapf(storage.CheckSQLDMLErr(query, err), "failed to add phone number %v", number)
}

func (u *users) generatePhoneValidationCode() string {
	return fmt.Sprintf("%04d", rand.Intn(9999-1)+1) //nolint:gomnd,gosec // Do we need super random here?
}

func (u *users) sendValidationCode(number, code string) error {
	fmt.Println(number, code)
	//nolint:nolintlint    // TODO Here we send SMS to phone number.
	return nil
}

func (u *users) updatePhoneValidationCode(ctx context.Context, id UserID, number string) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "update phone number failed because context failed")
	}

	validationCode := u.generatePhoneValidationCode()

	if err := u.sendValidationCode(number, validationCode); err != nil {
		return errors.Wrapf(err, "failed to send validation code to phone number %v", number)
	}

	sql := fmt.Sprintf(`UPDATE %v SET validation_code = :validationCode, phone_number = :phoneNumber, created_at = :createdAt WHERE id = :userID`, tableCodes)

	params := map[string]interface{}{
		"userID":         id,
		"validationCode": validationCode,
		"phoneNumber":    number,
		"createdAt":      time.Now().UTC().UnixNano(),
	}

	query, err := u.db.PrepareExecute(sql, params)

	return errors.Wrapf(storage.CheckSQLDMLErr(query, err), "failed to update phone number %v", number)
}

func (p *phoneNumberValidationCodes) ConfirmPhoneNumber(ctx context.Context, conf *PhoneNumberConfirm) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "check phone code failed because context failed")
	}
	result := new(phoneNumberValidationCode)

	pk := fmt.Sprintf("pk_unnamed_%v_1", tableCodes)
	if err := p.db.GetTyped(tableCodes, pk, []interface{}{conf.ID}, result); err != nil {
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
