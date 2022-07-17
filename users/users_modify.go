// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:funlen // Barely over the limit. It becomes uglier if broken down even further.
func (r *repository) ModifyUser(ctx context.Context, arg *ModifyUserArg) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "update user failed because context failed")
	}
	if arg.User.Country != "" && !r.IsValid(arg.User.Country) {
		return ErrInvalidCountry
	}
	gUser, err := r.getUserByID(ctx, arg.User.ID)
	if err != nil {
		return errors.Wrapf(err, "get user %v failed", arg.User.ID)
	}
	if err = r.triggerNewPhoneNumberValidation(ctx, &arg.User, gUser); err != nil {
		return errors.Wrapf(err, "failed to trigger new phone number validation for %#v", arg)
	}
	if arg.ProfilePicture != nil {
		arg.ProfilePicture.Filename = fmt.Sprintf("%v", gUser.HashCode)
		arg.User.ProfilePictureURL = fmt.Sprintf("%v/%v", cfg.PictureStorage.URLDownload, arg.ProfilePicture.Filename)
	}
	if err = r.uploadProfilePicture(ctx, arg.ProfilePicture); err != nil {
		return errors.Wrapf(err, "failed to upload profile picture for userID:%v", arg.User.ID)
	}
	sql, params := arg.genSQLUpdate()
	if len(params) == 1+1 {
		return nil
	}
	if err = storage.CheckSQLDMLErr(r.db.PrepareExecute(sql, params)); err != nil {
		_, err = detectAndParseDuplicateDatabaseError(err)

		return errors.Wrapf(err, "failed to update user %#v", arg)
	}
	us := &UserSnapshot{User: gUser.override(&arg.User), Before: gUser}
	arg.User = *us.User

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
func (arg *ModifyUserArg) genSQLUpdate() (sql string, params map[string]interface{}) {
	params = make(map[string]interface{})
	usr := arg.User
	params["id"] = usr.ID
	params["updatedAt"] = time.Now()

	sql = "UPDATE USERS set UPDATED_AT = :updatedAt"

	if usr.Email != "" {
		params["email"] = usr.Email
		sql += ", EMAIL = :email"
	}
	if usr.FirstName != "" {
		params["firstName"] = usr.FirstName
		sql += ", FIRST_NAME = :firstName"
	}
	if usr.LastName != "" {
		params["lastName"] = usr.LastName
		sql += ", LAST_NAME = :lastName"
	}
	if usr.Username != "" {
		params["username"] = usr.Username
		sql += ", USERNAME = :username"
	}
	if arg.ProfilePicture != nil {
		params["profilePictureName"] = arg.ProfilePicture.Filename
		sql += ", PROFILE_PICTURE_NAME = :profilePictureName"
	}
	if usr.Country != "" {
		params["country"] = usr.Country
		sql += ", COUNTRY = :country"
	}
	if usr.City != "" {
		params["city"] = usr.City
		sql += ", CITY = :city"
	}
	if arg.confirmedPhoneNumber != "" {
		// Updating phone number.
		params["phoneNumber"] = arg.confirmedPhoneNumber
		sql += ", PHONE_NUMBER = :phoneNumber"
		// And its hash, we need hashes to know if users are in agenda for each other.
		params["phoneNumberHash"] = usr.PhoneNumberHash
		sql += ", PHONE_NUMBER_HASH = :phoneNumberHash"
	}
	// Agenda can be updated after user creation (in case if user granted permission to access contacts on the team screen after initial user created).
	if usr.AgendaPhoneNumberHashes != "" {
		params["agendaPhoneNumberHashes"] = usr.AgendaPhoneNumberHashes
		sql += ", AGENDA_PHONE_NUMBER_HASHES = :agendaPhoneNumberHashes"
	}
	sql += " WHERE ID = :id"

	return sql, params
}
