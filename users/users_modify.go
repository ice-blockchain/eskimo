// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
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
	gUser, err := u.GetUser(ctx, user.ID)
	if err != nil {
		return errors.Wrapf(err, "get user failed")
	}
	// Phone number change or new number -> send SMS.
	if user.PhoneNumber != "" && user.PhoneNumber != gUser.PhoneNumber {
		confirm := new(PhoneNumberConfirmation)
		confirm.ID = user.ID
		confirm.PhoneNumber = user.PhoneNumber
		if err = u.updatePhoneValidationCode(ctx, confirm); err != nil {
			return errors.Wrapf(err, "update phone validation code failed")
		}
	}

	user.ProfilePicture.Filename = fmt.Sprintf("%v", gUser.HashCode)
	if err = u.uploadProfilePicture(ctx, &user.ProfilePicture); err != nil {
		return errors.Wrapf(err, "failed to upload profile picture")
	}

	sql, params := user.genSQLUpdate()
	query, err := u.db.PrepareExecute(sql, params)

	if err = storage.CheckSQLDMLErr(query, err); err != nil {
		return errors.Wrapf(err, "failed to update user with id %v", user.ID)
	}

	return errors.Wrap(u.sendUsersMessage(ctx, user), "failed to send updated user message")
}

//nolint:funlen // SQL large again
func (u *User) genSQLUpdate() (sql string, params map[string]interface{}) {
	params = make(map[string]interface{})
	params["id"] = u.ID
	params["updatedAt"] = time.Now().UTC().UnixNano()

	sql = fmt.Sprintf("UPDATE USERS set UPDATED_AT = :updatedAt")

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
		params["country"] = u.Country
		sql += ", COUNTRY = :country"
	}
	if u.confirmedPhoneNumber != "" {
		params["phoneNumber"] = u.confirmedPhoneNumber
		sql += ", PHONE_NUMBER = :phoneNumber"
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
