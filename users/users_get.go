// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
)

func (u *users) GetUser(ctx context.Context, id UserID) (*User, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user failed because context failed")
	}
	result := new(user)

	if err := u.db.GetTyped("USERS", "pk_unnamed_USERS_1", []interface{}{id}, result); err != nil {
		return nil, errors.Wrapf(err, "failed to get user by id %v", id)
	}

	if result.ID == "" {
		return nil, errors.Wrapf(ErrNotFound, "no user found with id %v", id)
	}

	return result.toUser(), nil
}

func (u *users) UsernameExists(ctx context.Context, name Username) (bool, error) {
	if ctx.Err() != nil {
		return false, errors.Wrap(ctx.Err(), "get user failed because context failed")
	}
	result := new(user)

	if err := u.db.GetTyped("USERS", "USERS_USERNAME_LOOKUP_IX", []interface{}{name}, result); err != nil {
		return false, errors.Wrapf(err, "failed to get user")
	}

	return result.ID != "", nil
}

func (u *user) toUser() *User {
	profilePictureURL := fmt.Sprintf("%s/%s", cfg.PictureStorage.URLDownload, u.ProfilePictureName)

	return &User{
		ID:                u.ID,
		HashCode:          u.HashCode,
		ReferredBy:        u.ReferredBy,
		Username:          u.Username,
		Email:             u.Email,
		FullName:          u.FullName,
		PhoneNumber:       u.PhoneNumber,
		ProfilePictureURL: profilePictureURL,
	}
}
