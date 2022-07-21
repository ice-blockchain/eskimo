// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"
	"mime/multipart"
	"strings"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:funlen // It needs a better breakdown.
func (r *repository) ModifyUser(ctx context.Context, usr *User, profilePicture *multipart.FileHeader) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "update user failed because context failed")
	}
	if usr.Country != "" && !r.IsValid(usr.Country) {
		return ErrInvalidCountry
	}
	gUser, err := r.getUserByID(ctx, usr.ID)
	if err != nil {
		return errors.Wrapf(err, "get user %v failed", usr.ID)
	}
	usr.UpdatedAt = time.Now()
	if err = r.triggerNewPhoneNumberValidation(ctx, usr, gUser); err != nil {
		return errors.Wrapf(err, "failed to trigger new phone number validation for %#v", usr)
	}
	if profilePicture != nil {
		lastDotIdx := strings.LastIndex(profilePicture.Filename, ".")
		var ext string
		if lastDotIdx > 0 {
			ext = profilePicture.Filename[lastDotIdx:]
		}
		profilePicture.Filename = fmt.Sprintf("%v%v", gUser.HashCode, ext)
		usr.ProfilePictureURL = profilePicture.Filename
		if err = r.uploadProfilePicture(ctx, profilePicture); err != nil {
			return errors.Wrapf(err, "failed to upload profile picture for userID:%v", usr.ID)
		}
	}
	sql, params := usr.genSQLUpdate(ctx)
	if len(params) == 1+1 {
		return nil
	}
	if err = storage.CheckSQLDMLErr(r.db.PrepareExecute(sql, params)); err != nil {
		_, err = detectAndParseDuplicateDatabaseError(err)

		return errors.Wrapf(err, "failed to update user %#v", usr)
	}
	if profilePicture != nil {
		usr.setCorrectProfilePictureURL()
	}
	us := &UserSnapshot{User: gUser.override(usr), Before: gUser}
	*usr = *us.User

	return errors.Wrapf(r.sendUserSnapshotMessage(ctx, us), "failed to send updated user snapshot message %#v", us)
}

func (u *User) override(user *User) *User {
	mergeField := func(oldData, newData string) string {
		if newData != "" {
			return newData
		}

		return oldData
	}

	usr := new(User)
	*usr = *u
	usr.UpdatedAt = user.UpdatedAt
	usr.Email = mergeField(u.Email, user.Email)
	usr.FirstName = mergeField(u.FirstName, user.FirstName)
	usr.LastName = mergeField(u.LastName, user.LastName)
	usr.Username = mergeField(u.Username, user.Username)
	usr.ProfilePictureURL = mergeField(u.ProfilePictureURL, user.ProfilePictureURL)
	usr.Country = mergeField(u.Country, user.Country)
	usr.City = mergeField(u.City, user.City)
	usr.PhoneNumber = mergeField(u.PhoneNumber, user.PhoneNumber)
	usr.PhoneNumberHash = mergeField(u.PhoneNumberHash, user.PhoneNumberHash)
	usr.AgendaPhoneNumberHashes = mergeField(u.AgendaPhoneNumberHashes, user.AgendaPhoneNumberHashes)

	return usr
}

//nolint:funlen // Because it's a big unitary SQL processing logic.
func (u *User) genSQLUpdate(ctx context.Context) (sql string, params map[string]interface{}) {
	params = make(map[string]interface{})
	params["id"] = u.ID
	params["updatedAt"] = u.UpdatedAt

	sql = "UPDATE USERS set UPDATED_AT = :updatedAt"

	if u.Email != "" {
		params["email"] = u.Email
		sql += ", EMAIL = :email"
	}
	if u.FirstName != "" {
		params["firstName"] = u.FirstName
		sql += ", FIRST_NAME = :firstName"
	}
	if u.LastName != "" {
		params["lastName"] = u.LastName
		sql += ", LAST_NAME = :lastName"
	}
	if u.Username != "" {
		params["username"] = u.Username
		sql += ", USERNAME = :username"
	}
	if u.ProfilePictureURL != "" {
		params["profilePictureName"] = u.ProfilePictureURL
		sql += ", PROFILE_PICTURE_NAME = :profilePictureName"
	}
	if u.Country != "" {
		params["country"] = u.Country
		sql += ", COUNTRY = :country"
	}
	if u.City != "" {
		params["city"] = u.City
		sql += ", CITY = :city"
	}
	if isPhoneNumberConfirmed, ok := ctx.Value(isPhoneNumberConfirmedCtxValueKey).(bool); ok && isPhoneNumberConfirmed {
		// Updating phone number.
		params["phoneNumber"] = u.PhoneNumber
		sql += ", PHONE_NUMBER = :phoneNumber"
		// And its hash, we need hashes to know if users are in agenda for each other.
		params["phoneNumberHash"] = u.PhoneNumberHash
		sql += ", PHONE_NUMBER_HASH = :phoneNumberHash"
	}
	// Agenda can be updated after user creation (in case if user granted permission to access contacts on the team screen after initial user created).
	if u.AgendaPhoneNumberHashes != "" {
		params["agendaPhoneNumberHashes"] = u.AgendaPhoneNumberHashes
		sql += ", AGENDA_PHONE_NUMBER_HASHES = :agendaPhoneNumberHashes"
	}
	sql += " WHERE ID = :id"

	return sql, params
}
