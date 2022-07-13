// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"text/template"
	stdlibtime "time"

	"github.com/cenkalti/backoff/v4"
	"github.com/framey-io/go-tarantool"
	"github.com/pkg/errors"
	"github.com/twilio/twilio-go"
	twilioclient "github.com/twilio/twilio-go/client"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"

	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/terror"
	"github.com/ice-blockchain/wintr/time"
)

func initTwilioClient() *twilio.RestClient {
	return twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: cfg.PhoneNumberValidation.TwilioCredentials.User,
		Password: cfg.PhoneNumberValidation.TwilioCredentials.Password,
	})
}

func (*repository) generatePhoneNumberValidationCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(999999)) //nolint:gomnd // static value
	log.Panic(errors.Wrapf(err, "crypto random generator failed"))

	return fmt.Sprintf("%06d", n.Uint64())
}

func (r *repository) sendValidationSMS(ctx context.Context, number, code string) error {
	var smsMessageBuffer bytes.Buffer
	smsTemplate := template.Must(new(template.Template).Parse(cfg.PhoneNumberValidation.SmsTemplate))
	smsTemplateData := map[string]interface{}{
		"code":           code,
		"expirationTime": cfg.PhoneNumberValidation.ExpirationTime.Minutes(),
	}
	log.Panic(errors.Wrapf(smsTemplate.Execute(&smsMessageBuffer, smsTemplateData), "invalid SMS template"))

	return errors.Wrapf(retry(ctx, func() error {
		if ctx.Err() != nil {
			//nolint:wrapcheck // It's a proxy.
			return backoff.Permanent(ctx.Err())
		}
		_, err := r.twilioClient.Api.CreateMessage(new(openapi.CreateMessageParams).
			SetTo(number).
			SetFrom(cfg.PhoneNumberValidation.FromPhoneNumber).
			SetBody(smsMessageBuffer.String()))

		//nolint:wrapcheck // It's wrapped outside.
		return err
	}), "failed to send sms message via twilio")
}

func (r *repository) validatePhoneNumber(number string) (string, error) {
	lookupResponse, err := r.twilioClient.LookupsV1.FetchPhoneNumber(number, nil)
	if err != nil {
		//nolint:errorlint // errors.As(err,*twilioclient.TwilioRestError) doesn't seem to work.
		if tErr, ok := err.(*twilioclient.TwilioRestError); !ok || tErr.Code != 20404 || tErr.Status != 404 {
			return "", errors.Wrapf(err, "failed to validate and lookup phone number %v", number)
		}

		return "", ErrInvalidPhoneNumber
	}
	if lookupResponse.PhoneNumber != nil {
		return *lookupResponse.PhoneNumber, nil
	}

	return number, nil
}

//nolint:gocognit // .
func (r *repository) replacePhoneValidation(ctx context.Context, conf *PhoneNumberValidation) error {
	sql := `REPLACE INTO PHONE_NUMBER_VALIDATIONS (USER_ID, PHONE_NUMBER, PHONE_NUMBER_HASH, VALIDATION_CODE, CREATED_AT) 
						 VALUES 				  (:id,     :phoneNumber, :phoneNumberHash,  :validationCode, :createdAt)`
	if conf.CreatedAt == nil {
		conf.CreatedAt = time.Now()
	}
	params := map[string]interface{}{"id": conf.UserID, "phoneNumber": conf.PhoneNumber, "phoneNumberHash": conf.PhoneNumberHash, "createdAt": conf.CreatedAt}
	needSms := false
	for ctx.Err() == nil {
		if conf.ValidationCode == "" {
			needSms = true
			conf.ValidationCode = r.generatePhoneNumberValidationCode()
		}
		params["validationCode"] = conf.ValidationCode
		err := storage.CheckSQLDMLErr(r.db.PrepareExecute(sql, params))
		if err != nil && errors.Is(err, ErrDuplicate) {
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed updating phone number validation for userID:%v ", conf.UserID)
		}
		if err == nil {
			break
		}
	}
	if !needSms {
		return nil
	}

	return errors.Wrapf(r.sendValidationSMS(ctx, conf.PhoneNumber, conf.ValidationCode), "failed to send validation SMS")
}

func (r *repository) ValidatePhoneNumber(ctx context.Context, conf *PhoneNumberValidation) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "check phone validation code failed because context failed")
	}
	result, err := r.getPhoneNumberValidation(ctx, conf.UserID)

	switch {
	case err != nil:
		return errors.Wrapf(err, "failed to getPhoneNumberValidation by userID: %v", conf.UserID)
	case result.UserID == "":
		return errors.Wrapf(ErrNotFound, "user  with id %v has no phone number waiting for confirmation", conf.UserID)
	case result.PhoneNumber != conf.PhoneNumber:
		return errors.Wrapf(ErrNotFound, "no phone number %v waiting for confirmation", conf.PhoneNumber)
	case result.PhoneNumberHash != conf.PhoneNumberHash:
		return errors.Wrapf(ErrNotFound, "phone number hash %v does not match initial hash", conf.PhoneNumberHash)
	case result.ValidationCode != conf.ValidationCode:
		return ErrInvalidPhoneValidationCode
	case stdlibtime.Duration(time.Now().UnixNano()-result.CreatedAt.UnixNano()) > cfg.PhoneNumberValidation.ExpirationTime:
		return ErrExpiredPhoneValidationCode
	}
	conf.CreatedAt = result.CreatedAt

	return errors.Wrapf(r.modifyPhoneNumber(ctx, conf), "error modifying the phone number for the user %v", conf.UserID)
}

func (r *repository) modifyPhoneNumber(ctx context.Context, conf *PhoneNumberValidation) error {
	arg := new(ModifyUserArg)
	arg.User.ID = conf.UserID
	arg.User.PhoneNumberHash = conf.PhoneNumberHash
	arg.confirmedPhoneNumber = conf.PhoneNumber
	if err := r.ModifyUser(ctx, arg); err != nil {
		if errors.Is(err, ErrNotFound) {
			err = ErrRelationNotFound
		}

		return errors.Wrapf(err, "error modifying phone number in users for userID:%v", arg.User.ID)
	}
	confirm := new(PhoneNumberValidation)
	confirm.UserID = arg.User.ID
	confirm.PhoneNumber = conf.PhoneNumber
	confirm.PhoneNumberHash = conf.PhoneNumberHash
	confirm.ValidationCode = arg.User.ID

	return errors.Wrapf(r.replacePhoneValidation(ctx, confirm), "error updating phone number validation for userID:%v", conf.UserID)
}

func (r *repository) getPhoneNumberValidation(ctx context.Context, id UserID) (*PhoneNumberValidation, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "context failed")
	}
	result := new(PhoneNumberValidation)
	err := r.db.GetTyped("PHONE_NUMBER_VALIDATIONS", "pk_unnamed_PHONE_NUMBER_VALIDATIONS_1", tarantool.StringKey{S: id}, result)

	return result, errors.Wrapf(err, "failed to get PhoneNumberValidation by userID: %v", id)
}

func (r *repository) triggerNewPhoneNumberValidation(ctx context.Context, newUser, oldUser *User) error {
	if newUser.PhoneNumber == "" || newUser.PhoneNumber == oldUser.PhoneNumber {
		return nil
	}

	phoneNumber, err := r.validatePhoneNumber(newUser.PhoneNumber)
	if err != nil {
		return errors.Wrapf(err, "invalid phone number %v", newUser.PhoneNumber)
	}
	if phoneNumber != newUser.PhoneNumber {
		return terror.New(ErrInvalidPhoneNumberFormat, map[string]interface{}{"phoneNumber": phoneNumber})
	}
	confirm := new(PhoneNumberValidation)
	confirm.UserID = newUser.ID
	confirm.PhoneNumber = newUser.PhoneNumber
	confirm.PhoneNumberHash = newUser.PhoneNumberHash

	return errors.Wrapf(r.replacePhoneValidation(ctx, confirm), "update phone validation failed for %#v", confirm)
}
