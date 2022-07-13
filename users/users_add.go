// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/zeebo/xxh3"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/terror"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:funlen,lll // A lot of SQL params.
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
		referral = `(SELECT X.ID FROM (SELECT X.ID FROM (SELECT ID FROM users WHERE ID != :id ORDER BY random() LIMIT 1) X UNION ALL SELECT :id as ID) X LIMIT 1)`
	}
	sql := fmt.Sprintf(`
	INSERT INTO users 
		(ID, HASH_CODE, EMAIL, FIRST_NAME, LAST_NAME, PHONE_NUMBER, PHONE_NUMBER_HASH, USERNAME, REFERRED_BY, PROFILE_PICTURE_NAME, COUNTRY, CITY, CREATED_AT, UPDATED_AT)
	VALUES
		(:id, :hashCode, :email, :firstName, :lastName, :phoneNumber, :phoneNumberHash, :username, %v, :profilePictureName, :country, :city, :createdAt, :updatedAt)`, referral)
	params := map[string]interface{}{
		"id":                 u.ID,
		"hashCode":           u.HashCode,
		"email":              u.Email,
		"firstName":          u.FirstName,
		"lastName":           u.LastName,
		"phoneNumber":        u.PhoneNumber,
		"phoneNumberHash":    u.PhoneNumberHash,
		"username":           u.Username,
		"profilePictureName": u.ProfilePictureURL,
		"country":            u.Country,
		"city":               u.City,
		"createdAt":          u.CreatedAt,
		"updatedAt":          u.UpdatedAt,
	}
	if u.ReferredBy != "" {
		params["referredBy"] = u.ReferredBy
	}
	if err := storage.CheckSQLDMLErr(r.db.PrepareExecute(sql, params)); err != nil {
		field, tErr := detectAndParseDuplicateDatabaseError(err)
		if field == "hash_code" {
			return r.CreateUser(ctx, arg)
		}

		return errors.Wrapf(tErr, "failed to insert user %#v", u)
	}

	return errors.Wrapf(r.sendUserSnapshotMessage(ctx, &UserSnapshot{User: &u, Before: nil}),
		"failed to send user created message for %#v", u)
}

func (r *repository) setCreateUserDefaults(ctx context.Context, arg *CreateUserArg) {
	arg.User.CreatedAt = time.Now()
	arg.User.UpdatedAt = arg.User.CreatedAt
	arg.User.DeviceLocation = *r.GetDeviceMetadataLocation(ctx, &GetDeviceMetadataLocationArg{ID: device.ID{UserID: arg.User.ID}, ClientIP: arg.ClientIP})
	arg.User.HashCode = xxh3.HashStringSeed(arg.User.ID, uint64(arg.User.CreatedAt.UnixNano()))
	arg.User.ProfilePictureURL = defaultUserImage
}

func detectAndParseDuplicateDatabaseError(err error) (field string, _ error) {
	if tErr := terror.As(err); tErr != nil && errors.Is(err, storage.ErrDuplicate) {
		switch tErr.Data[storage.IndexName] {
		case "pk_unnamed_USERS_1":
			field = "id"
		case "unique_unnamed_USERS_2":
			field = "username"
		case "unique_unnamed_USERS_3":
			field = "hash_code"
		default:
			log.Panic("unexpected indexName `%v` for users space", tErr.Data[storage.IndexName])
		}
		err = terror.New(storage.ErrDuplicate, map[string]interface{}{"field": field})
	}

	return field, err //nolint:wrapcheck // It's a proxy.
}
