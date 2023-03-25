// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"fmt"
	"net"
	stdlibtime "time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/zeebo/xxh3"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/terror"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:funlen,lll // A lot of SQL params.
func (r *repository) CreateUser(ctx context.Context, usr *User, clientIP net.IP) (err error) {
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
		(:id, :miningBlockchainAccountAddress, :blockchainAccountAddress, :hashCode, :email, :firstName, :lastName, :phoneNumber, :phoneNumberHash, :username, :referredBy, :randomReferredBy, :clientData, :profilePictureName, :country, :city, :language, :createdAt, :updatedAt)`
	params := map[string]any{
		"id":                             usr.ID,
		"hashCode":                       usr.HashCode,
		"miningBlockchainAccountAddress": usr.MiningBlockchainAccountAddress,
		"blockchainAccountAddress":       usr.BlockchainAccountAddress,
		"email":                          usr.Email,
		"firstName":                      usr.FirstName,
		"lastName":                       usr.LastName,
		"phoneNumber":                    usr.PhoneNumber,
		"phoneNumberHash":                usr.PhoneNumberHash,
		"username":                       usr.Username,
		"profilePictureName":             usr.ProfilePictureURL,
		"country":                        usr.Country,
		"city":                           usr.City,
		"language":                       usr.Language,
		"createdAt":                      usr.CreatedAt,
		"updatedAt":                      usr.UpdatedAt,
		"referredBy":                     usr.ReferredBy,
		"randomReferredBy":               *usr.RandomReferredBy,
		"clientData":                     usr.ClientData,
	}
	if err = storage.CheckSQLDMLErr(r.db.PrepareExecute(sql, params)); err != nil {
		field, tErr := detectAndParseDuplicateDatabaseError(err)
		if field == hashCodeDBColumnName || field == usernameDBColumnName || errors.Is(err, storage.ErrRelationNotFound) {
			return r.CreateUser(ctx, usr, clientIP)
		}

		return errors.Wrapf(tErr, "failed to insert user %#v", usr)
	}
	us := &UserSnapshot{User: r.sanitizeUser(usr), Before: nil}
	if err = r.sendUserSnapshotMessage(ctx, us); err != nil {
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
	usr.HashCode = xxh3.HashStringSeed(usr.ID, uint64(usr.CreatedAt.UnixNano()))
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

func detectAndParseDuplicateDatabaseError(err error) (field string, newErr error) {
	newErr = err
	if tErr := terror.As(newErr); tErr != nil && errors.Is(newErr, storage.ErrDuplicate) {
		switch tErr.Data[storage.IndexName] {
		case "unique_unnamed_USERS_1":
			field = "phoneNumber"
		case "unique_unnamed_USERS_2":
			field = "email"
		case "pk_unnamed_USERS_3":
			field = "id"
		case "unique_unnamed_USERS_4":
			field = usernameDBColumnName
		case "unique_unnamed_USERS_5":
			field = "phoneNumberHash"
		case "unique_unnamed_USERS_6":
			field = "mining_blockchain_account_address"
		case "unique_unnamed_USERS_7":
			field = "blockchainAccountAddress"
		case "unique_unnamed_USERS_8":
			field = hashCodeDBColumnName
		default:
			log.Panic("unexpected indexName `%v` for users space", tErr.Data[storage.IndexName])
		}
		newErr = terror.New(storage.ErrDuplicate, map[string]any{"field": field})
	}

	return field, newErr //nolint:wrapcheck // It's a proxy.
}
