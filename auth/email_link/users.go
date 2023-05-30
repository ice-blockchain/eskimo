// SPDX-License-Identifier: ice License 1.0

package emaillink

import (
	"context"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
)

func (r *repository) getUserByEmail(ctx context.Context, email, oldEmail string) (*minimalUser, error) {
	userID, err := r.findOrGenerateUserIDByEmail(ctx, email, oldEmail)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch or generate userID")
	}
	user, err := r.getUserByIDOrEmail(ctx, userID, email)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user info by id %v", userID)
	}

	return user, nil
}

func (r *repository) findOrGenerateUserIDByEmail(ctx context.Context, email, oldEmail string) (userID string, err error) {
	if ctx.Err() != nil {
		return "", errors.Wrap(ctx.Err(), "find or generate user by id or email context failed")
	}
	randomID := uuid.NewString()
	type dbUserID struct {
		ID users.UserID
	}
	searchEmail := email
	if oldEmail != "" {
		searchEmail = oldEmail
	}
	ids, err := storage.Select[dbUserID](ctx, r.db, `SELECT id FROM users WHERE email=$1 OR id = $2`, searchEmail, randomID)
	if err != nil || len(ids) == 0 {
		if storage.IsErr(err, storage.ErrNotFound) || len(ids) == 0 {
			return randomID, nil
		}

		return "", errors.Wrapf(err, "failed to find user by userID:%v or email:%v", randomID, email)
	}
	if ids[0].ID == randomID || (len(ids) > 1) {
		return r.findOrGenerateUserIDByEmail(ctx, email, oldEmail)
	}

	return ids[0].ID, nil
}

func (r *repository) getUserByIDOrEmail(ctx context.Context, id users.UserID, email string) (*minimalUser, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user by id of email failed because context failed")
	}
	result, err := storage.Get[minimalUser](ctx, r.db, `
		WITH emails AS (
			SELECT $1 as id, email, COALESCE((custom_claims -> 'hash_code')::BIGINT,0) as hash_code, custom_claims FROM email_confirmations WHERE email = $2
		)
		SELECT u.id, u.email, u.hash_code, emails.custom_claims as custom_claims FROM users u, emails WHERE u.id = $1
		UNION ALL (select * from emails)
		LIMIT 1
	`, id, email)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user by id:%v or email:%v", id, email)
	}

	return result, nil
}
