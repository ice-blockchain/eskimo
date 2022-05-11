// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"

	"github.com/pkg/errors"
)

// refers to the USERS space, so placing it to the users package.
func (u *users) GetTier1Referrals(ctx context.Context, id UserID, limit, offset uint64) ([]*User, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "failed to get referrals because of context failed")
	}

	var queryResult []*user
	sql := `SELECT * FROM USERS WHERE referred_by = :user_id ORDER BY created_at DESC LIMIT :limit OFFSET :offset`
	params := map[string]interface{}{
		"user_id": id,
		"limit":   limit,
		"offset":  offset,
	}
	if err := u.db.PrepareExecuteTyped(sql, params, &queryResult); err != nil {
		return nil, errors.Wrap(err, "failed to get T1 referrals")
	}
	result := make([]*User, 0, len(queryResult))
	for _, referral := range queryResult {
		result = append(result, referral.toUser())
	}

	return result, nil
}
