package users

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/zeebo/xxh3"

	"github.com/ICE-Blockchain/wintr/connectors/storage"
)

//nolint:funlen // A lot of SQL
func (u *users) AddUser(ctx context.Context, user *User) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "add user failed because context failed")
	}
	user.created()

	var refer UserID
	if user.ReferredBy != "" {
		refer = fmt.Sprintf("'%v'", user.ReferredBy)
	} else {
		refer = `(SELECT ID FROM users LIMIT 1)`
	}

	sql := fmt.Sprintf(`INSERT INTO users (ID, HASH_CODE, EMAIL, FULL_NAME, PHONE_NUMBER,
	USERNAME, REFERRED_BY, PROFILE_PICTURE_NAME, COUNTRY, CREATED_AT, UPDATED_AT)
	VALUES(:id, :hashCode, :email, :fullName, :phoneNumber,
	:username, %v, :profilePictureName, :country, :createdAt, :updatedAt)`, refer)

	params := map[string]interface{}{
		"id":                 user.ID,
		"hashCode":           u.hash(user.ID),
		"email":              user.Email,
		"fullName":           user.FullName,
		"phoneNumber":        user.PhoneNumber,
		"username":           user.Username,
		"profilePictureName": defaultUserImage,
		"country":            user.Country,
		"createdAt":          user.CreatedAt.UnixNano(),
		"updatedAt":          user.UpdatedAt.UnixNano(),
	}

	query, err := u.db.PrepareExecute(sql, params)
	if err = storage.CheckSQLDMLErr(query, err); err != nil {
		return errors.Wrapf(err, "failed to add user %#v", user)
	}

	u.sendUsersMessage(ctx, user)

	return nil
}

func (u *users) hash(data string) uint64 {
	return xxh3.HashStringSeed(data, uint64(time.Now().UTC().UnixNano()))
}

func (u *User) created() *User {
	u.CreatedAt = time.Now().UTC()
	u.UpdatedAt = u.CreatedAt

	return u
}
