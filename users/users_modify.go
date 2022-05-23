// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/imroc/req/v3"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/log"
)

func (u *users) ModifyUser(ctx context.Context, user *User) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "update user failed because context failed")
	}
	gUser, err := u.GetUserByID(ctx, user.ID)
	if err != nil {
		return errors.Wrapf(err, "get user failed")
	}
	if err = u.triggerNewPhoneNumberValidation(ctx, user, gUser); err != nil {
		return errors.Wrap(err, "failed to trigger new phone number validation")
	}

	user.ProfilePicture.Filename = fmt.Sprintf("%v", gUser.HashCode)
	if err = u.uploadProfilePicture(ctx, &user.ProfilePicture); err != nil {
		return errors.Wrapf(err, "failed to upload profile picture")
	}

	sql, params := user.genSQLUpdate()
	if len(params) == 1+1 {
		return nil
	}
	query, err := u.db.PrepareExecute(sql, params)

	if err = storage.CheckSQLDMLErr(query, err); err != nil {
		return errors.Wrapf(err, "failed to update user with id %v", user.ID)
	}

	return errors.Wrap(u.sendUsersMessage(ctx, UserSnapshot{User: gUser.override(user), Before: gUser}), "failed to send updated user message")
}

func (u *User) override(user *User) *User {
	mergeField := func(oldData, newData string) string {
		if newData != "" {
			return newData
		}

		return oldData
	}

	n := new(User)
	*n = *u
	n.UpdatedAt = user.UpdatedAt
	n.Email = mergeField(u.Email, user.Email)
	n.FullName = mergeField(u.FullName, user.FullName)
	n.Username = mergeField(u.Username, user.Username)
	n.ProfilePicture.Filename = mergeField(u.ProfilePicture.Filename, user.ProfilePicture.Filename)
	n.Country = mergeField(u.Country, user.Country)
	n.PhoneNumber = mergeField(u.PhoneNumber, user.PhoneNumber)

	return n
}

func (u *users) triggerNewPhoneNumberValidation(ctx context.Context, newUser, oldUser *User) error {
	if newUser.PhoneNumber == "" || newUser.PhoneNumber == oldUser.PhoneNumber {
		return nil
	}

	pn, err := u.validatePhoneNumber(newUser.PhoneNumber)
	if err != nil {
		return errors.Wrapf(err, "invalid phone number")
	}

	confirm := new(PhoneNumberConfirmation)
	confirm.UserID = newUser.ID
	confirm.PhoneNumber = pn
	confirm.PhoneNumberHash = newUser.PhoneNumberHash

	err = u.updatePhoneValidationCode(ctx, confirm)

	return errors.Wrapf(err, "update phone validation code failed")
}

//nolint:funlen // SQL large again
func (u *User) genSQLUpdate() (sql string, params map[string]interface{}) {
	params = make(map[string]interface{})
	params["id"] = u.ID
	params["updatedAt"] = time.Now().UTC().UnixNano()

	sql = "UPDATE USERS set UPDATED_AT = :updatedAt"

	if u.Email != "" {
		params["email"] = u.Email
		sql += ", EMAIL = :email"
	}
	if u.FullName != "" {
		params["fullName"] = u.FullName
		sql += ", FULL_NAME = :fullName"
	}
	if u.Username != "" {
		params["username"] = u.Username
		sql += ", USERNAME = :username"
	}
	if u.ProfilePicture.Filename != "" {
		params["profilePictureName"] = u.ProfilePicture.Filename
		sql += ", PROFILE_PICTURE_NAME = :profilePictureName"
	}
	if u.Country != "" {
		params["country"] = strings.ToLower(u.Country)
		sql += ", COUNTRY = :country"
	}
	if u.confirmedPhoneNumber != "" {
		// Updating phone number.
		params["phoneNumber"] = u.confirmedPhoneNumber
		sql += ", PHONE_NUMBER = :phoneNumber"
		// And its hash, we need hashes to know if users are in agenda for each other.
		params["phoneNumberHash"] = u.PhoneNumberHash
		sql += ", PHONE_NUMBER_HASH = :phoneNumberHash"
	}
	// Agenda can be updated after user creation (in case if user granted permission to access contacts on the team screen after initial user created).
	if u.AgendaPhoneNumberHashes != "" {
		params["agendaPhoneNumberHashes"] = u.AgendaPhoneNumberHashes
		sql += ", AGENDA_PHONE_NUMBER_HASHES = :agendaPhoneNumberHashes"
	}
	sql += " WHERE ID = :id"

	return sql, params
}

func (u *users) uploadProfilePicture(ctx context.Context, data *multipart.FileHeader) error {
	if data.Size == 0 {
		return nil
	}
	file, err := data.Open()
	defer func(file multipart.File) {
		err = file.Close()
		if err != nil {
			log.Error(err, "error closing file")
		}
	}(file)
	if err != nil {
		return errors.Wrap(err, "error opening file")
	}
	fileData, err := io.ReadAll(file)
	if err != nil {
		return errors.Wrap(err, "error reading file")
	}

	url := fmt.Sprintf("%s/%s", cfg.PictureStorage.URLUpload, data.Filename)
	_, err = req.
		SetContext(ctx).
		SetRetryCount(3). //nolint:gomnd // Static config
		SetRetryCondition(func(resp *req.Response, err error) bool {
			return (err != nil) || (resp.StatusCode == http.StatusTooManyRequests)
		}).
		SetHeader("AccessKey", cfg.PictureStorage.AccessKey).
		SetHeader("Content-Type", data.Header.Get("Content-Type")).
		SetBodyBytes(fileData).Put(url)

	return errors.Wrap(err, "error uploading file")
}
