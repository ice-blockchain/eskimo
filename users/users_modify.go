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

	"github.com/ICE-Blockchain/wintr/connectors/storage"
	"github.com/ICE-Blockchain/wintr/log"
)

func (u *users) ModifyUser(ctx context.Context, user *User) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "update user failed because context failed")
	}
	user.updated()

	gUser, err := u.GetUser(ctx, user.ID)
	if err != nil {
		return errors.Wrapf(ErrNotFound, "no user found with id %v", user.ID)
	}

	if user.ProfilePicture.Filename != "" && user.ProfilePicture.Size != 0 {
		user.ProfilePicture.Filename = fmt.Sprintf("%v", gUser.HashCode)
		err = u.uploadProfilePicture(ctx, &user.ProfilePicture)
		if err != nil {
			return errors.Wrapf(err, "failed to upload user image, id %v", user.ID)
		}
	}

	sql, params := user.genSQLUpdate()
	query, err := u.db.PrepareExecute(sql, params)
	if err = storage.CheckSQLDMLErr(query, err); err != nil {
		return errors.Wrapf(err, "failed to update user with id %v", user.ID)
	}

	if err = u.sendUsersMessage(ctx, user); err != nil {
		log.Error(err, "Error sending user message")
	}

	return nil
}

//nolint:funlen // A lot of fields in DB table
func (u *User) genSQLUpdate() (sql string, params map[string]interface{}) {
	params = make(map[string]interface{})
	params["id"] = u.ID
	params["updatedAt"] = u.UpdatedAt.UnixNano()

	sql = fmt.Sprintf("UPDATE USERS set UPDATED_AT = :updatedAt")

	if u.Email != "" {
		params["email"] = u.Email
		sql += ", EMAIL = :email"
	}
	if u.FullName != "" {
		params["fullName"] = u.FullName
		sql += ", FULL_NAME = :fullName"
	}
	if u.PhoneNumber != "" {
		params["phoneNumber"] = u.PhoneNumber
		sql += ", PHONE_NUMBER = :phoneNumber"
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
	sql += " WHERE ID = :id"

	return sql, params
}

func (u *User) updated() *User {
	now := time.Now().UTC()
	u.UpdatedAt = now

	return u
}

func (u *users) uploadProfilePicture(ctx context.Context, data *multipart.FileHeader) error {
	file, err := data.Open()
	defer file.Close()
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
	if err != nil {
		return errors.Wrap(err, "error uploading file")
	}

	return nil
}
