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
	agendaBefore, agendaContactIDsForUpdate, uniqueAgendaContactIDsForSend, err := r.findAgendaContactIDs(ctx, usr)
	sql, params := usr.genSQLUpdate(ctx, agendaContactIDsForUpdate)
	noOpNoOfParams := 1 + 1
	if lu != nil || usr.AgendaPhoneNumberHashes != "" {
		noOpNoOfParams++
	}
	if len(params) == noOpNoOfParams {
		*usr = *r.sanitizeUser(oldUsr)
		usr.sanitizeForUI()

		return nil
	}
	if updatedRowsCount, tErr := storage.Exec(ctx, r.db, sql, params...); tErr != nil {
		_, tErr = detectAndParseDuplicateDatabaseError(tErr)
		if !storage.IsErr(tErr, storage.ErrDuplicate) && (storage.IsErr(tErr, storage.ErrNotFound) || updatedRowsCount == 0) {
			return ErrRaceCondition
		}

		return errors.Wrapf(err, "failed to update user %#v", usr)
	}
	bkpUsr := *oldUsr
	if profilePicture != nil {
		bkpUsr.ProfilePictureURL = RandomDefaultProfilePictureName()
	}
	if sErr := runConcurrently(ctx, r.sendContactMessage, uniqueAgendaContactIDsForSend); sErr != nil {
		_, rollBackParams := bkpUsr.genSQLUpdate(ctx, agendaBefore)
		rollBackParams[1] = bkpUsr.UpdatedAt.Time
		_, rErr := storage.Exec(ctx, r.db, sql, rollBackParams...)

		return errors.Wrapf(multierror.Append(rErr, sErr).ErrorOrNil(), "can't send contacts message for userID:%v", usr.ID)
	}

	us := &UserSnapshot{User: r.sanitizeUser(oldUsr.override(usr)), Before: r.sanitizeUser(oldUsr)}
	if err = r.sendUserSnapshotMessage(ctx, us); err != nil {
		_, rollBackParams := bkpUsr.genSQLUpdate(ctx, agendaBefore)
		rollBackParams[1] = bkpUsr.UpdatedAt.Time
		_, rollbackErr := storage.Exec(ctx, r.db, sql, rollBackParams...)

		return multierror.Append( //nolint:wrapcheck // Not needed.
			errors.Wrapf(err, "failed to send updated user snapshot message %#v", us),
			errors.Wrapf(rollbackErr, "failed to replace user to previous value, due to rollback, prev:%#v", bkpUsr),
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
	usr.KYCPassed = mergePointerField(u.KYCPassed, user.KYCPassed)
	usr.ClientData = mergePointerToMapField(u.ClientData, user.ClientData)
	usr.ReferredBy = mergeStringField(u.ReferredBy, user.ReferredBy)
	usr.Email = mergeStringField(u.Email, user.Email)
	usr.FirstName = mergePointerField(u.FirstName, user.FirstName)
	usr.LastName = mergePointerField(u.LastName, user.LastName)
	usr.Username = mergeStringField(u.Username, user.Username)
	usr.ProfilePictureURL = mergeStringField(u.ProfilePictureURL, user.ProfilePictureURL)
	usr.Country = mergeStringField(u.Country, user.Country)
	usr.City = mergeStringField(u.City, user.City)
	usr.Language = mergeStringField(u.Language, user.Language)
	usr.PhoneNumber = mergeStringField(u.PhoneNumber, user.PhoneNumber)
	usr.PhoneNumberHash = mergeStringField(u.PhoneNumberHash, user.PhoneNumberHash)
	usr.BlockchainAccountAddress = mergeStringField(u.BlockchainAccountAddress, user.BlockchainAccountAddress)

	return usr
}

//nolint:funlen,gocognit,gocyclo,revive,cyclop // Because it's a big unitary SQL processing logic.
func (u *User) genSQLUpdate(ctx context.Context, agendaUserIDs []UserID) (sql string, params []any) {
	params = make([]any, 0)
	params = append(params, u.ID, u.UpdatedAt.Time)

	sql = "UPDATE users SET updated_at = $2"
	nextIndex := 3
	if u.LastMiningStartedAt != nil {
		params = append(params, u.LastMiningStartedAt.Time)
		sql += fmt.Sprintf(", LAST_MINING_STARTED_AT = $%v", nextIndex)
		nextIndex++
	}
	if u.LastMiningEndedAt != nil {
		params = append(params, u.LastMiningEndedAt.Time)
		sql += fmt.Sprintf(", LAST_MINING_ENDED_AT = $%v", nextIndex)
		nextIndex++
	}
	if u.LastPingCooldownEndedAt != nil {
		params = append(params, u.LastPingCooldownEndedAt.Time)
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
		nextIndex++
	}
	if u.RandomReferredBy != nil {
		params = append(params, u.RandomReferredBy)
		sql += fmt.Sprintf(", RANDOM_REFERRED_BY = $%v", nextIndex)
		nextIndex++
	}
	if u.ClientData != nil {
		params = append(params, u.ClientData)
		sql += fmt.Sprintf(", CLIENT_DATA = $%v::json", nextIndex)
		nextIndex++
	}
	if u.FirstName != nil && *u.FirstName != "" {
		params = append(params, u.FirstName)
		sql += fmt.Sprintf(", FIRST_NAME = $%v", nextIndex)
		nextIndex++
	}
	if u.LastName != nil && *u.LastName != "" {
		params = append(params, u.LastName)
		sql += fmt.Sprintf(", LAST_NAME = $%v", nextIndex)
		nextIndex++
	}
	if u.Username != "" {
		params = append(params, u.Username)
		sql += fmt.Sprintf(", USERNAME = $%v", nextIndex)
		params = append(params, u.lookup())
		sql += fmt.Sprintf(", LOOKUP = $%v::tsvector", nextIndex+1)
		nextIndex += 2
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
	if u.BlockchainAccountAddress != "" {
		params = append(params, u.BlockchainAccountAddress)
		sql += fmt.Sprintf(", BLOCKCHAIN_ACCOUNT_ADDRESS = $%v", nextIndex)
		nextIndex++
	}
	if u.AgendaPhoneNumberHashes != "" {
		params = append(params, agendaUserIDs)
		sql += fmt.Sprintf(", agenda_contact_user_ids = $%v", nextIndex)
		nextIndex++
	}

	sql += " WHERE ID = $1"

	if lu := lastUpdatedAt(ctx); lu != nil {
		params = append(params, lu.Time)
		sql += fmt.Sprintf(" AND UPDATED_AT = $%v", nextIndex)
	}

	return sql, params
}

func (u *User) lookup() string {
	return strings.ToLower(strings.Join(generateUsernameKeywords(u.Username), " "))
}

func resolveProfilePictureExtension(fileName string) string {
	lastDotIdx := strings.LastIndex(fileName, ".")
	var ext string
	if lastDotIdx > 0 {
		ext = fileName[lastDotIdx:]
	}

	return ext
}
