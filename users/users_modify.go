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

	n := new(User)
	*n = *u
	n.UpdatedAt = user.UpdatedAt
	n.Email = mergeField(u.Email, user.Email)
	n.FirstName = mergeField(u.FirstName, user.FirstName)
	n.LastName = mergeField(u.LastName, user.LastName)
	n.Username = mergeField(u.Username, user.Username)
	n.ProfilePictureURL = mergeField(u.ProfilePictureURL, user.ProfilePictureURL)
	n.Country = mergeField(u.Country, user.Country)
	n.City = mergeField(u.City, user.City)
	n.PhoneNumber = mergeField(u.PhoneNumber, user.PhoneNumber)
	n.PhoneNumberHash = mergeField(u.PhoneNumberHash, user.PhoneNumberHash)
	n.AgendaPhoneNumberHashes = mergeField(u.AgendaPhoneNumberHashes, user.AgendaPhoneNumberHashes)

	return n
}

//nolint:funlen // Because it's a big unitary SQL processing logic.
func (arg *ModifyUserArg) genSQLUpdate() (sql string, params map[string]interface{}) {
	params = make(map[string]interface{})
	u := arg.User
	params["id"] = u.ID
	params["updatedAt"] = time.Now()

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
	if arg.ProfilePicture != nil {
		params["profilePictureName"] = arg.ProfilePicture.Filename
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
	if arg.confirmedPhoneNumber != "" {
		// Updating phone number.
		params["phoneNumber"] = arg.confirmedPhoneNumber
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
