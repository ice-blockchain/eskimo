// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/zeebo/xxh3"

	"github.com/ICE-Blockchain/wintr/connectors/storage"
)

//nolint:funlen // A lot of SQL
func (u *users) AddUser(ctx context.Context, user *User) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "add user failed because context failed")
	}
	user.created()

	var referral UserID
	if user.ReferredBy != "" {
		referral = ":referredBy"
	} else {
		referral = `(SELECT ID FROM users ORDER BY random() LIMIT 1)`
	}

	sql := fmt.Sprintf(`INSERT INTO users (ID, HASH_CODE, EMAIL, FULL_NAME, PHONE_NUMBER,
	USERNAME, REFERRED_BY, PROFILE_PICTURE_NAME, COUNTRY, CREATED_AT, UPDATED_AT)
	VALUES(:id, :hashCode, :email, :fullName, :phoneNumber,
	:username, %v, :profilePictureName, :country, :createdAt, :updatedAt)`, referral)

	params := map[string]interface{}{
		"id":                 user.ID,
		"hashCode":           u.hash(user.ID),
		"email":              user.Email,
		"fullName":           user.FullName,
		"username":           user.Username,
		"profilePictureName": defaultUserImage,
		"country":            user.Country,
		"createdAt":          user.CreatedAt.UnixNano(),
		"updatedAt":          user.UpdatedAt.UnixNano(),
	}

	if user.ReferredBy != "" {
		params["referredBy"] = user.ReferredBy
	}

	query, err := u.db.PrepareExecute(sql, params)
	if err = storage.CheckSQLDMLErr(query, err); err != nil {
		return errors.Wrapf(err, "failed to add user %#v", user)
	}

	if err = u.AddPhoneValidationCode(ctx, user.ID, user.PhoneNumber); err != nil {
		return errors.Wrap(err, "failed to add users phone number")
	}

	return errors.Wrap(u.sendUsersMessage(ctx, user), "failed to send user created message")
}

func (u *users) hash(data string) uint64 {
	return xxh3.HashStringSeed(data, uint64(time.Now().UTC().UnixNano()))
}

func (u *User) created() *User {
	u.CreatedAt = time.Now().UTC()
	u.UpdatedAt = u.CreatedAt

	return u
}
