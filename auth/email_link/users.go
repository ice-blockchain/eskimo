// SPDX-License-Identifier: ice License 1.0

package emaillink

import (
	"context"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
)

func (r *repository) findOrGenerateUserIDByEmail(ctx context.Context, email string) (userID, userEmail string, err error) {
	if ctx.Err() != nil {
		return "", "", errors.Wrap(ctx.Err(), "context failed")
	}
	randomID := uuid.NewString()
	type dbUserID struct {
		ID users.UserID
	}
	ids, err := storage.Select[dbUserID](ctx, r.db, `SELECT id FROM users WHERE email=$1 OR id = $2`, email, randomID)
	if err != nil || len(ids) == 0 {
		if storage.IsErr(err, storage.ErrNotFound) || len(ids) == 0 {
			return randomID, email, nil
		}

		return "", "", errors.Wrapf(err, "failed to search for existing userId for email: %v", email)
	}
	if ids[0].ID == randomID || (len(ids) > 1) {
		return r.findOrGenerateUserIDByEmail(ctx, email)
	}

	return ids[0].ID, email, nil
}

func (r *repository) getUserByID(ctx context.Context, id users.UserID) (*users.User, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user failed because context failed")
	}
	result, err := storage.Get[users.User](ctx, r.db, `SELECT * FROM users u WHERE id = $1`, id)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user by id %v", id)
	}

	return result, nil
}
