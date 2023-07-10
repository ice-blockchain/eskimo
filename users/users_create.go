// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"net"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

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
	r.setCreateUserDefaults(ctx, usr, clientIP)
	sql := `
	INSERT INTO users 
		(ID, MINING_BLOCKCHAIN_ACCOUNT_ADDRESS, BLOCKCHAIN_ACCOUNT_ADDRESS, EMAIL, FIRST_NAME, LAST_NAME, PHONE_NUMBER, PHONE_NUMBER_HASH, USERNAME, REFERRED_BY, RANDOM_REFERRED_BY, CLIENT_DATA, PROFILE_PICTURE_NAME, COUNTRY, CITY, LANGUAGE, CREATED_AT, UPDATED_AT, LOOKUP)
	VALUES
		($1,                                $2,                         $3,    $4,         $5,        $6,           $7,                $8,       $9,         $10,                $11,   $12::json,                  $13,     $14,  $15,      $16,        $17,        $18,    $19::tsvector)`
	args := []any{
		usr.ID, usr.MiningBlockchainAccountAddress, usr.BlockchainAccountAddress, usr.Email, usr.FirstName, usr.LastName,
		usr.PhoneNumber, usr.PhoneNumberHash, usr.Username, usr.ReferredBy, usr.RandomReferredBy, usr.ClientData, usr.ProfilePictureURL, usr.Country,
		usr.City, usr.Language, usr.CreatedAt.Time, usr.UpdatedAt.Time, usr.lookup(),
	}
	if _, err := storage.Exec(ctx, r.db, sql, args...); err != nil {
		field, tErr := detectAndParseDuplicateDatabaseError(err)
		if field == usernameDBColumnName {
			return r.CreateUser(ctx, usr, clientIP)
		}
		if storage.IsErr(err, storage.ErrRelationNotFound) {
			return errors.Wrapf(ErrNotFound, "no such userID:%v for referred_by for user:%v", usr.ReferredBy, usr.ID)
		}

		return errors.Wrapf(tErr, "failed to insert user %#v", usr)
	}
	us := &UserSnapshot{User: r.sanitizeUser(usr), Before: nil}
	if err := errors.Wrapf(r.sendUserSnapshotMessage(ctx, us), "failed to send user created message for %#v", usr); err != nil {
		revertCtx, revertCancel := context.WithTimeout(context.Background(), requestDeadline)
		defer revertCancel()

		return multierror.Append(errors.Wrapf(err, "failed to send user created message for %#v", usr), //nolint:wrapcheck // Not needed.
			errors.Wrapf(r.deleteUser(revertCtx, usr), "failed to delete user due to rollback, for userID:%v", usr.ID)).ErrorOrNil() //nolint:contextcheck // .
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
	usr.ProfilePictureURL = RandomDefaultProfilePictureName()
	usr.Username = usr.ID
	if usr.ReferredBy == "" {
		usr.ReferredBy = usr.ID
	}
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
	if storage.IsErr(err, storage.ErrDuplicate) { //nolint:nestif // .
		if storage.IsErr(err, storage.ErrDuplicate, "pk") { //nolint:gocritic // .
			field = "id"
		} else if storage.IsErr(err, storage.ErrDuplicate, "phonenumber") {
			field = "phone_number"
		} else if storage.IsErr(err, storage.ErrDuplicate, "email") {
			field = "email"
		} else if storage.IsErr(err, storage.ErrDuplicate, usernameDBColumnName) {
			field = usernameDBColumnName
		} else if storage.IsErr(err, storage.ErrDuplicate, "phonenumberhash") {
			field = "phoneNumberHash"
		} else if storage.IsErr(err, storage.ErrDuplicate, "miningblockchainaccountaddress") {
			field = "mining_blockchain_account_address"
		} else if storage.IsErr(err, storage.ErrDuplicate, "blockchainaccountaddress") {
			field = "blockchainAccountAddress"
		} else {
			log.Panic("unexpected duplicate field for users space: %v", err)
		}

		return field, terror.New(storage.ErrDuplicate, map[string]any{"field": field})
	}

	return "", err
}
