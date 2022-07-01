// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"
	"net"

	"github.com/goccy/go-json"
	"github.com/pkg/errors"
	"github.com/zeebo/xxh3"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/terror"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:funlen,lll // A lot of SQL params.
func (r *repository) CreateUser(ctx context.Context, usr *User, clientIP net.IP) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "create user failed because context failed")
	}
	r.setCreateUserDefaults(ctx, usr, clientIP)
	var referral UserID
	if usr.ReferredBy != "" {
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
		"id":                 usr.ID,
		"hashCode":           usr.HashCode,
		"email":              usr.Email,
		"firstName":          usr.FirstName,
		"lastName":           usr.LastName,
		"phoneNumber":        usr.PhoneNumber,
		"phoneNumberHash":    usr.PhoneNumberHash,
		"username":           usr.Username,
		"profilePictureName": usr.ProfilePictureURL,
		"country":            usr.Country,
		"city":               usr.City,
		"createdAt":          usr.CreatedAt,
		"updatedAt":          usr.UpdatedAt,
	}
	if usr.ReferredBy != "" {
		params["referredBy"] = usr.ReferredBy
	}
	if err := storage.CheckSQLDMLErr(r.db.PrepareExecute(sql, params)); err != nil {
		field, tErr := detectAndParseDuplicateDatabaseError(err)
		if field == hashCodeDBColumnName {
			return r.CreateUser(ctx, usr, clientIP)
		}
		usrBytes, err := json.Marshal(usr)
		usrStr := string(usrBytes)
		if err != nil {
			usrStr = fmt.Sprintf("%#v", usr)
		}
		return errors.Wrapf(tErr, "failed to insert user %v", usrStr)
	}
	usr.setCorrectProfilePictureURL()

	return errors.Wrapf(r.sendUserSnapshotMessage(ctx, &UserSnapshot{User: usr, Before: nil}),
		"failed to send user created message for %#v", usr)
}

func (r *repository) setCreateUserDefaults(ctx context.Context, usr *User, clientIP net.IP) {
	usr.CreatedAt = time.Now()
	usr.UpdatedAt = usr.CreatedAt
	usr.DeviceLocation = *r.GetDeviceMetadataLocation(ctx, device.ID{UserID: usr.ID}, clientIP)
	usr.HashCode = xxh3.HashStringSeed(usr.ID, uint64(usr.CreatedAt.UnixNano()))
	usr.ProfilePictureURL = defaultUserImage
}

func detectAndParseDuplicateDatabaseError(err error) (field string, newErr error) {
	newErr = err
	if tErr := terror.As(newErr); tErr != nil && errors.Is(newErr, storage.ErrDuplicate) {
		switch tErr.Data[storage.IndexName] {
		case "pk_unnamed_USERS_1":
			field = "id"
		case "unique_unnamed_USERS_2":
			field = "username"
		case "unique_unnamed_USERS_3":
			field = hashCodeDBColumnName
		default:
			log.Panic("unexpected indexName `%v` for users space", tErr.Data[storage.IndexName])
		}
		newErr = terror.New(storage.ErrDuplicate, map[string]interface{}{"field": field})
	}

	return field, newErr //nolint:wrapcheck // It's a proxy.
}
