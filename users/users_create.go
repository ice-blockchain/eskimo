// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"fmt"
	"net"
	stdlibtime "time"

	"github.com/pkg/errors"
	"github.com/zeebo/xxh3"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/terror"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:funlen,lll // A lot of SQL params.
func (r *repository) CreateUser(ctx context.Context, usr *User, clientIP net.IP) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "create user failed because context failed")
	}
	before2 := time.Now()
	defer func() {
		if elapsed := stdlibtime.Since(*before2.Time); elapsed > 100*stdlibtime.Millisecond {
			log.Info(fmt.Sprintf("[response]CreateUser took: %v", elapsed))
		}
	}()
	r.setCreateUserDefaults(ctx, usr, clientIP)
	sql := `
	INSERT INTO users 
		(ID, MINING_BLOCKCHAIN_ACCOUNT_ADDRESS, BLOCKCHAIN_ACCOUNT_ADDRESS, HASH_CODE, EMAIL, FIRST_NAME, LAST_NAME, PHONE_NUMBER, PHONE_NUMBER_HASH, USERNAME, REFERRED_BY, RANDOM_REFERRED_BY, CLIENT_DATA, PROFILE_PICTURE_NAME, COUNTRY, CITY, LANGUAGE, CREATED_AT, UPDATED_AT)
	VALUES
		($1,                                $2,                         $3,        $4,    $5,         $6,        $7,           $8,                $9,      $10,         $11,                $12,  $13::json,                   $14,     $15,  $16,      $17,        $18,        $19)`
	args := []any{
		usr.ID, usr.MiningBlockchainAccountAddress, usr.BlockchainAccountAddress, usr.HashCode, usr.Email, usr.FirstName, usr.LastName,
		usr.PhoneNumber, usr.PhoneNumberHash, usr.Username, usr.ReferredBy, usr.RandomReferredBy, usr.ClientData, usr.ProfilePictureURL, usr.Country,
		usr.City, usr.Language, usr.CreatedAt.Time, usr.UpdatedAt.Time,
	}
	if _, err := storage.Exec(ctx, r.dbV2, sql, args...); err != nil {
		field, tErr := detectAndParseDuplicateDatabaseError(err)
		if field == hashCodeDBColumnName || field == usernameDBColumnName || errors.Is(err, storage.ErrRelationNotFound) {
			return r.CreateUser(ctx, usr, clientIP)
		}

		return errors.Wrapf(tErr, "failed to insert user %#v", usr)
	}
	us := &UserSnapshot{User: r.sanitizeUser(usr), Before: nil}
	if err := errors.Wrapf(r.sendUserSnapshotMessage(ctx, us), "failed to send user created message for %#v", usr); err != nil {
		return errors.Wrapf(err, "failed to send user created message for %#v", usr)
	}
	hashCode := usr.HashCode
	usr.sanitizeForUI()
	usr.HashCode = hashCode

	return nil
}

func (r *repository) setCreateUserDefaults(ctx context.Context, usr *User, clientIP net.IP) {
	usr.CreatedAt = time.Now()
	usr.UpdatedAt = usr.CreatedAt
	usr.DeviceLocation = *r.GetDeviceMetadataLocation(ctx, &device.ID{UserID: usr.ID}, clientIP)
	usr.HashCode = int64(xxh3.HashStringSeed(usr.ID, uint64(usr.CreatedAt.UnixNano())))
	usr.ProfilePictureURL = RandomDefaultProfilePictureName()
	usr.Username = usr.ID
	usr.ReferredBy = usr.ID
	if usr.MiningBlockchainAccountAddress == "" {
		usr.MiningBlockchainAccountAddress = usr.ID
	}
	if usr.Language == "" {
		usr.Language = "en"
	}
	if usr.BlockchainAccountAddress == "" {
		usr.BlockchainAccountAddress = usr.ID
	}
	if usr.PhoneNumber == "" {
		usr.PhoneNumber, usr.PhoneNumberHash = usr.ID, usr.ID
	}
	if usr.Email == "" {
		usr.Email = usr.ID
	}
	randomReferredBy := false
	usr.RandomReferredBy = &randomReferredBy
}

func detectAndParseDuplicateDatabaseError(err error) (field string, newErr error) { //nolint:revive // need to check all fields in this way.
	if storage.IsErr(err, storage.ErrDuplicate) {

		if storage.IsErr(err, storage.ErrDuplicate, "pk") { //nolint:gocritic,nestif // .
			field = "id"
		} else if storage.IsErr(err, storage.ErrDuplicate, "phonenumber") {
			field = "phone_number"
		} else if storage.IsErr(err, storage.ErrDuplicate, "email") {
			field = "email"
		} else if storage.IsErr(err, storage.ErrDuplicate, usernameDBColumnName) {
			field = usernameDBColumnName
		} else if storage.IsErr(err, storage.ErrDuplicate, "phonenumberhash") {
			field = "phone_number_hash"
		} else if storage.IsErr(err, storage.ErrDuplicate, "miningblockchainaccountaddress") {
			field = "mining_blockchain_account_address"
		} else if storage.IsErr(err, storage.ErrDuplicate, "blockchainaccountaddress") {
			field = "blockchain_account_address"
		} else if storage.IsErr(err, storage.ErrDuplicate, "hashcode") {
			field = hashCodeDBColumnName
		} else {
			log.Panic("unexpected duplicate field for users space: %v", err)
		}

		return field, terror.New(storage.ErrDuplicate, map[string]any{"field": field})
	}

	return "", err
}
