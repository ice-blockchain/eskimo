// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage"
)

func (r *repository) DeleteUser(ctx context.Context, userID UserID) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "delete user failed because context failed")
	}
	gUser, err := r.getUserByID(ctx, userID)
	if err != nil {
		return errors.Wrapf(err, "unable to get user %v", userID)
	}
	if err = storage.CheckSQLDMLErr(r.db.PrepareExecute(`DELETE FROM users WHERE id = :id`, map[string]interface{}{"id": userID})); err != nil {
		return errors.Wrapf(err, "failed to delete user with id %v", userID)
	}
	u := &UserSnapshot{User: nil, Before: gUser}

	return errors.Wrapf(r.sendUserSnapshotMessage(ctx, u), "failed to send deleted user message for %#v", u)
}
