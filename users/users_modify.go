// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"fmt"
	"mime/multipart"
	"strings"
	stdlibtime "time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	storage "github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:funlen,gocognit,gocyclo,revive,cyclop // It needs a better breakdown.
func (r *repository) ModifyUser(ctx context.Context, usr *User, profilePicture *multipart.FileHeader) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "update user failed because context failed")
	}
	before2 := time.Now()
	defer func() {
		if elapsed := stdlibtime.Since(*before2.Time); elapsed > 100*stdlibtime.Millisecond {
			log.Info(fmt.Sprintf("[response]ModifyUser took: %v", elapsed))
		}
	}()
	oldUsr, err := r.getUserByID(ctx, usr.ID)
	if err != nil {
		return errors.Wrapf(err, "get user %v failed", usr.ID)
	}
	lu := lastUpdatedAt(ctx)
	if lu != nil && oldUsr.UpdatedAt.UnixNano() != lu.UnixNano() {
		return ErrRaceCondition
	}
	if usr.Country != "" && !r.IsValid(usr.Country) {
		return ErrInvalidCountry
	}
	if usr.Language != "" && oldUsr.Language == usr.Language {
		usr.Language = ""
	}
	if usr.LastPingCooldownEndedAt != nil && oldUsr.LastPingCooldownEndedAt != nil && oldUsr.LastPingCooldownEndedAt.Equal(*usr.LastPingCooldownEndedAt.Time) {
		usr.LastPingCooldownEndedAt = nil
	}
	if usr.ReferredBy != "" { // TO.DO::Remove this once the issue mentioned in `updateReferredBy` gets fixed.
		if err = r.updateReferredBy(ctx, oldUsr, usr.ReferredBy, false); err != nil {
			return errors.Wrapf(err, "failed to updateReferredBy to %v for userID %v", usr.ReferredBy, oldUsr.ID)
		}
		usr.ReferredBy = ""
		ctx = ContextWithChecksum(ctx, oldUsr.Checksum()) //nolint:revive // Not an issue here.
	}
	usr.UpdatedAt = time.Now()
	if profilePicture != nil {
		if profilePicture.Header.Get("Reset") == "true" {
			profilePicture.Filename = RandomDefaultProfilePictureName()
		} else {
			pictureExt := resolveProfilePictureExtension(profilePicture.Filename)
			profilePicture.Filename = fmt.Sprintf("%v_%v%v", oldUsr.HashCode, usr.UpdatedAt.UnixNano(), pictureExt)
		}
		usr.ProfilePictureURL = profilePicture.Filename
		if err = r.pictureClient.UploadPicture(ctx, profilePicture, oldUsr.ProfilePictureURL); err != nil {
			return errors.Wrapf(err, "failed to upload profile picture for userID:%v", usr.ID)
		}
	}
	sql, params := usr.genSQLUpdate(ctx)
	noOpNoOfParams := 1 + 1
	if lu != nil {
		noOpNoOfParams++
	}
	if len(params) == noOpNoOfParams {
		*usr = *r.sanitizeUser(oldUsr)
		usr.sanitizeForUI()

		return nil
	}
	if updatedRowsCount, err := storage.Exec(ctx, r.dbV2, sql, params...); err != nil {
		if errors.Is(err, storage.ErrNotFound) || updatedRowsCount == 0 {
			return ErrRaceCondition
		}
		_, err = detectAndParseDuplicateDatabaseError(err)

		return errors.Wrapf(err, "failed to update user %#v", usr)
	}
	bkpUsr := *oldUsr
	if profilePicture != nil {
		bkpUsr.ProfilePictureURL = RandomDefaultProfilePictureName()
	}
	us := &UserSnapshot{User: r.sanitizeUser(oldUsr.override(usr)), Before: r.sanitizeUser(oldUsr)}
	if err = r.sendUserSnapshotMessage(ctx, us); err != nil {
		return multierror.Append( //nolint:wrapcheck // Not needed.
			errors.Wrapf(err, "failed to send updated user snapshot message %#v", us),
			errors.Wrapf(r.db.ReplaceTyped("USERS", &bkpUsr, &[]*User{}), "failed to replace user to previous value, due to rollback, prev:%#v", bkpUsr),
		).ErrorOrNil()
	}
	*usr = *us.User
	usr.sanitizeForUI()

	return nil
}

func (u *User) override(user *User) *User {
	usr := new(User)
	*usr = *u
	usr.UpdatedAt = user.UpdatedAt
	usr.LastMiningStartedAt = mergeTimeField(u.LastMiningStartedAt, user.LastMiningStartedAt)
	usr.LastMiningEndedAt = mergeTimeField(u.LastMiningEndedAt, user.LastMiningEndedAt)
	usr.LastPingCooldownEndedAt = mergeTimeField(u.LastPingCooldownEndedAt, user.LastPingCooldownEndedAt)
	usr.HiddenProfileElements = mergePointerToArrayField(u.HiddenProfileElements, user.HiddenProfileElements)
	usr.RandomReferredBy = mergePointerField(u.RandomReferredBy, user.RandomReferredBy)
	usr.Verified = mergePointerField(u.Verified, user.Verified)
	usr.ClientData = mergePointerToMapField(u.ClientData, user.ClientData)
	usr.ReferredBy = mergeStringField(u.ReferredBy, user.ReferredBy)
	usr.Email = mergeStringField(u.Email, user.Email)
	usr.FirstName = mergeStringField(u.FirstName, user.FirstName)
	usr.LastName = mergeStringField(u.LastName, user.LastName)
	usr.Username = mergeStringField(u.Username, user.Username)
	usr.ProfilePictureURL = mergeStringField(u.ProfilePictureURL, user.ProfilePictureURL)
	usr.Country = mergeStringField(u.Country, user.Country)
	usr.City = mergeStringField(u.City, user.City)
	usr.Language = mergeStringField(u.Language, user.Language)
	usr.PhoneNumber = mergeStringField(u.PhoneNumber, user.PhoneNumber)
	usr.PhoneNumberHash = mergeStringField(u.PhoneNumberHash, user.PhoneNumberHash)
	usr.AgendaPhoneNumberHashes = mergeStringField(u.AgendaPhoneNumberHashes, user.AgendaPhoneNumberHashes)
	usr.BlockchainAccountAddress = mergeStringField(u.BlockchainAccountAddress, user.BlockchainAccountAddress)

	return usr
}

//nolint:funlen,gocognit,gocyclo,revive,cyclop // Because it's a big unitary SQL processing logic.
func (u *User) genSQLUpdate(ctx context.Context) (sql string, params []any) {
	params = make([]any, 0)
	params = append(params, u.ID, u.UpdatedAt)

	sql = "UPDATE users SET updated_at = $2"
	nextIndex := 2
	if u.LastMiningStartedAt != nil {
		params = append(params, u.LastMiningStartedAt)
		sql += fmt.Sprintf(", LAST_MINING_STARTED_AT = $%v", nextIndex)
		nextIndex++
	}
	if u.LastMiningEndedAt != nil {
		params = append(params, u.LastMiningEndedAt)
		sql += fmt.Sprintf(", LAST_MINING_ENDED_AT = $%v", nextIndex)
		nextIndex++
	}
	if u.LastPingCooldownEndedAt != nil {
		params = append(params, u.LastPingCooldownEndedAt)
		sql += fmt.Sprintf(", LAST_PING_COOLDOWN_ENDED_AT = $%v", nextIndex)
		nextIndex++
	}
	if u.HiddenProfileElements != nil {
		params = append(params, u.HiddenProfileElements)
		sql += fmt.Sprintf(", HIDDEN_PROFILE_ELEMENTS = $%v", nextIndex)
		nextIndex++
	}
	if u.ReferredBy != "" {
		params = append(params, u.ReferredBy)
		sql += fmt.Sprintf(", REFERRED_BY = $%v", nextIndex)
		falseVal := false
		u.RandomReferredBy = &falseVal
	}
	if u.RandomReferredBy != nil {
		params = append(params, u.RandomReferredBy)
		sql += fmt.Sprintf(", RANDOM_REFERRED_BY = $%v", nextIndex)
		nextIndex++
	}
	if u.ClientData != nil {
		params = append(params, u.ClientData)
		sql += fmt.Sprintf(", CLIENT_DATA = $%v", nextIndex)
		nextIndex++
	}
	if u.FirstName != "" {
		params = append(params, u.FirstName)
		sql += fmt.Sprintf(", FIRST_NAME = $%v", nextIndex)
		nextIndex++
	}
	if u.LastName != "" {
		params = append(params, u.LastName)
		sql += fmt.Sprintf(", LAST_NAME = $%v", nextIndex)
		nextIndex++
	}
	if u.Username != "" {
		params = append(params, u.Username)
		sql += fmt.Sprintf(", USERNAME = $%v", nextIndex)
		nextIndex++
	}
	if u.ProfilePictureURL != "" {
		params = append(params, u.ProfilePictureURL)
		sql += fmt.Sprintf(", PROFILE_PICTURE_NAME = $%v", nextIndex)
		nextIndex++
	}
	if u.Country != "" {
		params = append(params, u.Country)
		sql += fmt.Sprintf(", COUNTRY = $%v", nextIndex)
		nextIndex++
	}
	if u.City != "" {
		params = append(params, u.City)
		sql += fmt.Sprintf(", CITY = $%v", nextIndex)
		nextIndex++
	}
	if u.Language != "" {
		params = append(params, u.Language)
		sql += fmt.Sprintf(", LANGUAGE = $%v", nextIndex)
		nextIndex++
	}
	if u.PhoneNumber != "" {
		params = append(params, u.PhoneNumber)
		sql += fmt.Sprintf(", PHONE_NUMBER = $%v", nextIndex)
		params = append(params, u.PhoneNumberHash)
		sql += fmt.Sprintf(", PHONE_NUMBER_HASH = $%v", nextIndex+1)
		nextIndex += 2
	}
	if u.Email != "" {
		params = append(params, u.Email)
		sql += fmt.Sprintf(", EMAIL = $%v", nextIndex)
		nextIndex++
	}
	// Agenda can be updated after user creation (in case if user granted permission to access contacts on the team screen after initial user created).
	if u.AgendaPhoneNumberHashes != "" {
		params = append(params, u.AgendaPhoneNumberHashes)
		sql += fmt.Sprintf(", AGENDA_PHONE_NUMBER_HASHES = $%v", nextIndex)
		nextIndex++
	}
	if u.BlockchainAccountAddress != "" {
		params = append(params, u.BlockchainAccountAddress)
		sql += fmt.Sprintf(", BLOCKCHAIN_ACCOUNT_ADDRESS = $%v", nextIndex)
		nextIndex++
	}
	sql += " WHERE ID = $1"

	if lu := lastUpdatedAt(ctx); lu != nil {
		params = append(params, lu)
		sql += fmt.Sprintf(" AND UPDATED_AT = $%v", nextIndex)
	}

	return sql, params
}

func resolveProfilePictureExtension(fileName string) string {
	lastDotIdx := strings.LastIndex(fileName, ".")
	var ext string
	if lastDotIdx > 0 {
		ext = fileName[lastDotIdx:]
	}

	return ext
}

//nolint:funlen // . TODO. replace this with `modifyUser` after this (https://github.com/tarantool/tarantool/issues/4661) gets resolved.
func (r *repository) updateReferredBy(ctx context.Context, usr *User, newReferredBy UserID, randomReferral bool) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	if _, err := r.getUserByID(ctx, newReferredBy); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			err = storage.ErrRelationNotFound
		}

		return errors.Wrapf(err, "get user %v failed", newReferredBy)
	}
	now := time.Now()
	//ops := append(make([]tarantool.Op, 0, 1+1+1),
	//	tarantool.Op{
	//		Op:    "=",
	//		Field: 6, //nolint:gomnd // random_referred_by.
	//		Arg:   randomReferral,
	//	}, tarantool.Op{
	//		Op:    "=",
	//		Field: 18, //nolint:gomnd // referred_by.
	//		Arg:   newReferredBy,
	//	}, tarantool.Op{
	//		Op:    "=",
	//		Field: 1, // It's updated_at.
	//		Arg:   now,
	//	})
	sql := `UPDATE users 
				SET random_referred_by = $1,
                    referred_by = $2,
                    updated_at = $3
                WHERE id = $4`
	if _, err := storage.Exec(ctx, r.dbV2, sql, randomReferral, newReferredBy, now.Time, usr.ID); err != nil {
		return errors.Wrapf(err, "failed to update random:%v referred_by to %v for userID %v", randomReferral, newReferredBy, usr.ID)
	}
	bkpUsr := *usr
	newUsr := *usr
	newUsr.ReferredBy = newReferredBy
	newUsr.UpdatedAt = now
	newUsr.RandomReferredBy = &randomReferral
	us := &UserSnapshot{User: r.sanitizeUser(&newUsr), Before: r.sanitizeUser(usr)}
	if err := r.sendUserSnapshotMessage(ctx, us); err != nil {
		return multierror.Append( //nolint:wrapcheck // Not needed.
			errors.Wrapf(err, "failed to send updated user message for %#v", us),
			errors.Wrapf(r.db.ReplaceTyped("USERS", &bkpUsr, &[]*User{}), "failed to replace user to previous value, due to rollback, prev:%#v", bkpUsr),
		).ErrorOrNil()
	}
	*usr = *us.User

	return nil
}
