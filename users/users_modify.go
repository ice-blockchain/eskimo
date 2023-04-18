// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"fmt"
	"math"
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
	if updatedRowsCount, tErr := storage.Exec(ctx, r.db, sql, params...); tErr != nil {
		_, tErr = detectAndParseDuplicateDatabaseError(tErr)
		if !storage.IsErr(tErr, storage.ErrDuplicate) && (storage.IsErr(tErr, storage.ErrNotFound) || updatedRowsCount == 0) {
			return ErrRaceCondition
		}

		return errors.Wrapf(err, "failed to update user %#v", usr)
	}

	if usr.AgendaPhoneNumberHashes != "" {
		if err = r.updateAgendaPhoneNumberHashes(ctx, usr, oldUsr); err != nil {
			return errors.Wrapf(err, "failed to insert agenda phone number hashes for userID:%v", usr.ID)
		}
	}

	bkpUsr := *oldUsr
	if profilePicture != nil {
		bkpUsr.ProfilePictureURL = RandomDefaultProfilePictureName()
	}
	us := &UserSnapshot{User: r.sanitizeUser(oldUsr.override(usr)), Before: r.sanitizeUser(oldUsr)}
	if err = r.sendUserSnapshotMessage(ctx, us); err != nil {
		_, rollBackParams := bkpUsr.genSQLUpdate(ctx)
		rollBackParams[1] = bkpUsr.UpdatedAt
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
		params = append(params, lu.Time)
		sql += fmt.Sprintf(" AND UPDATED_AT = $%v", nextIndex)
	}

	return sql, params
}

func (r *repository) updateAgendaPhoneNumberHashes(ctx context.Context, user, userBefore *User) error {
	removedContacts := userBefore.newlyAddedAgendaContacts(user)
	if len(removedContacts) == 0 {
		return r.processNewContacts(ctx, user, userBefore)
	}
	//nolint:makezero // We're know for sure.
	placeholders, values := make([]string, len(removedContacts)), make([]any, len(removedContacts)+1)
	idx := 0
	values[0] = user.ID
	const initialFields = 2
	for contact := range removedContacts {
		placeholders[idx] = fmt.Sprintf("$%v", idx+initialFields)
		values[idx+1] = contact
		idx++
	}
	sql := fmt.Sprintf("DELETE FROM agenda_phone_number_hashes WHERE user_id = $1 AND agenda_phone_number_hash in (%v)", strings.Join(placeholders, ","))
	if _, err := storage.Exec(ctx, r.db, sql, values...); err != nil {
		return errors.Wrapf(err, "failed to delete removed contacts for userId %v %#v", user.ID, removedContacts)
	}

	return r.processNewContacts(ctx, user, userBefore)
}

func (r *repository) processNewContacts(ctx context.Context, user, userBefore *User) error {
	newContacts := user.newlyAddedAgendaContacts(userBefore)
	if len(newContacts) == 0 {
		return nil
	}
	var (
		jx                     = 0
		allContactsBatches     = make([][]string, 0, (len(newContacts)/agendaPhoneNumberHashesBatchSize)+1)
		currentContactsBatches = make([]string, int(math.Min(float64(len(newContacts)), agendaPhoneNumberHashesBatchSize)))
	)
	for contact := range newContacts {
		if jx != 0 && jx%agendaPhoneNumberHashesBatchSize == 0 {
			allContactsBatches = append(allContactsBatches, append([]string{}, currentContactsBatches...))
		}
		currentContactsBatches[jx%agendaPhoneNumberHashesBatchSize] = contact
		jx++
	}
	allContactsBatches = append(allContactsBatches, currentContactsBatches)
	var mErr *multierror.Error

	for _, batch := range allContactsBatches {
		mErr = multierror.Append(mErr, r.insertAgendaContactsBatch(ctx, user.ID, batch))
	}

	return errors.Wrapf(mErr.ErrorOrNil(), "failed to insert new contacts in agenda for userID %v %#v", user.ID, newContacts)
}

func (r *repository) insertAgendaContactsBatch(ctx context.Context, userID UserID, contactsBatch []string) error {
	const fields = 2
	placeholders := make([]string, 0, len(contactsBatch))
	params := make([]any, len(contactsBatch)+1) //nolint:makezero // We're sure about size
	params[0] = userID
	for ix, contact := range contactsBatch {
		placeholders = append(placeholders, fmt.Sprintf("($1,$%v)", ix+fields))
		params[ix+1] = contact
	}
	sql := fmt.Sprintf(`INSERT INTO agenda_phone_number_hashes(user_id, agenda_phone_number_hash) VALUES %v
                                                                          ON CONFLICT(user_id, agenda_phone_number_hash) DO NOTHING`,
		strings.Join(placeholders, ","))
	_, err := storage.Exec(ctx, r.db, sql, params...)

	return errors.Wrapf(err, "failed to INSERT agenda_phone_number_hashes, params:%#v", params...)
}

func (u *User) newlyAddedAgendaContacts(beforeUser *User) map[string]struct{} { //nolint:gocognit,revive // .
	if u == nil || u.AgendaPhoneNumberHashes == "" || u.AgendaPhoneNumberHashes == u.ID {
		return nil
	}
	after := strings.Split(u.AgendaPhoneNumberHashes, ",")
	newlyAdded := make(map[string]struct{}, len(after))
	if beforeUser == nil || beforeUser.ID == "" || beforeUser.AgendaPhoneNumberHashes == "" || beforeUser.AgendaPhoneNumberHashes == beforeUser.ID {
		for _, agendaPhoneNumberHash := range after {
			if agendaPhoneNumberHash == "" {
				continue
			}
			newlyAdded[agendaPhoneNumberHash] = struct{}{}
		}

		return newlyAdded
	}
	before := strings.Split(beforeUser.AgendaPhoneNumberHashes, ",")
outer:
	for _, afterAgendaPhoneNumberHash := range after {
		if afterAgendaPhoneNumberHash == "" || strings.EqualFold(afterAgendaPhoneNumberHash, u.PhoneNumberHash) {
			continue
		}
		for _, beforeAgendaPhoneNumberHash := range before {
			if strings.EqualFold(beforeAgendaPhoneNumberHash, afterAgendaPhoneNumberHash) {
				continue outer
			}
		}
		newlyAdded[afterAgendaPhoneNumberHash] = struct{}{}
	}

	return newlyAdded
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
	sql := `UPDATE users 
				SET random_referred_by = $1,
                    referred_by = 		 $2,
                    updated_at = 		 $3
                WHERE id = $4`
	if _, err := storage.Exec(ctx, r.db, sql, randomReferral, newReferredBy, now.Time, usr.ID); err != nil {
		return errors.Wrapf(err, "failed to update random:%v referred_by to %v for userID %v", randomReferral, newReferredBy, usr.ID)
	}
	bkpUsr := *usr
	newUsr := *usr
	newUsr.ReferredBy = newReferredBy
	newUsr.UpdatedAt = now
	newUsr.RandomReferredBy = &randomReferral
	us := &UserSnapshot{User: r.sanitizeUser(&newUsr), Before: r.sanitizeUser(usr)}
	if err := r.sendUserSnapshotMessage(ctx, us); err != nil {
		_, rollBackParams := bkpUsr.genSQLUpdate(ctx)
		rollBackParams[1] = bkpUsr.UpdatedAt
		_, rollbackErr := storage.Exec(ctx, r.db, sql, rollBackParams...)

		return multierror.Append( //nolint:wrapcheck // Not needed.
			errors.Wrapf(err, "failed to send updated user message for %#v", us),
			errors.Wrapf(rollbackErr, "failed to replace user to previous value, due to rollback, prev:%#v", bkpUsr),
		).ErrorOrNil()
	}
	*usr = *us.User

	return nil
}
