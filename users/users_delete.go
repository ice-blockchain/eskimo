package users

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/ICE-Blockchain/wintr/connectors/storage"
)

func (u *users) RemoveUser(ctx context.Context, userID UserID) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "remove user failed because context failed")
	}
	gUser, err := u.GetUser(ctx, userID)
	if err != nil {
		return errors.Wrapf(err, "unable to get user %v", userID)
	}

	sql := `DELETE FROM users WHERE id = :id`
	params := map[string]interface{}{"id": userID}

	query, err := u.db.PrepareExecute(sql, params)
	if err = storage.CheckSQLDMLErr(query, err); err != nil {
		return errors.Wrapf(err, "failed to remove user with id %v", userID)
	}

	u.sendUsersMessage(ctx, gUser.deleted())

	return nil
}

func (u *User) deleted() *User {
	now := time.Now().UTC()
	u.DeletedAt = &now

	return u
}
