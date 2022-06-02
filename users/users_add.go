// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/zeebo/xxh3"

	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:funlen // A lot of SQL params.
func (r *repository) CreateUser(ctx context.Context, arg *CreateUserArg) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "create user failed because context failed")
	}
	r.setCreateUserDefaults(ctx, arg)
	var referral UserID
	u := arg.User
	if u.ReferredBy != "" {
		referral = ":referredBy"
	} else {
		referral = `(SELECT ID FROM users ORDER BY random() LIMIT 1)`
	}
	sql := fmt.Sprintf(`
	INSERT INTO users 
		(ID, HASH_CODE, EMAIL, FULL_NAME, PHONE_NUMBER, PHONE_NUMBER_HASH, USERNAME, REFERRED_BY, PROFILE_PICTURE_NAME, COUNTRY, CREATED_AT, UPDATED_AT)
	VALUES
		(:id, :hashCode, :email, :fullName, :phoneNumber, :phoneNumberHash, :username, %v, :profilePictureName, :country, :createdAt, :updatedAt)`, referral)
	params := map[string]interface{}{
		"id":                 u.ID,
		"hashCode":           u.HashCode,
		"email":              u.Email,
		"fullName":           u.FullName,
		"phoneNumber":        u.PhoneNumber,
		"phoneNumberHash":    u.PhoneNumberHash,
		"username":           u.Username,
		"profilePictureName": u.ProfilePictureURL,
		"country":            u.Country,
		"createdAt":          u.CreatedAt,
		"updatedAt":          u.UpdatedAt,
	}
	if u.ReferredBy != "" {
		params["referredBy"] = u.ReferredBy
	}
	if err := storage.CheckSQLDMLErr(r.db.PrepareExecute(sql, params)); err != nil {
		return errors.Wrapf(err, "failed to insert user %#v", u)
	}

	return errors.Wrapf(r.sendUserSnapshotMessage(ctx, &UserSnapshot{User: &u, Before: nil}),
		"failed to send user created message for %#v", u)
}

func (r *repository) setCreateUserDefaults(ctx context.Context, arg *CreateUserArg) {
	arg.User.CreatedAt = time.Now()
	arg.User.UpdatedAt = arg.User.CreatedAt
	arg.User.Country = r.GetDeviceCountry(ctx, arg.ClientIP)
	arg.User.HashCode = xxh3.HashStringSeed(arg.User.ID, uint64(arg.User.CreatedAt.UnixNano()))
	arg.User.ProfilePictureURL = defaultUserImage
}
