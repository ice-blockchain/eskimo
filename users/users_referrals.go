// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
)

// refers to the USERS space, so placing it to the users package.
func (u *users) GetTier1Referrals(ctx context.Context, id UserID, limit, offset uint64) ([]*Referral, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "failed to get referrals because of context failed")
	}

	var queryResult []*Referral
	sql := fmt.Sprintf(`SELECT u.id, u.username, u.phone_number, '%v/'||profile_picture_name AS profile_picture_name,`+
		`POSITION(u.PHONE_NUMBER_HASH_CODE ,(select AGENDA_PHONE_NUMBER_HASH_CODES from USERS where ID = :user_id")) > 0 as provided_in_agenda FROM USERS u`+
		`WHERE u.referred_by = :user_id ORDER BY provided_in_agenda DESC, u.created_at DESC LIMIT :limit OFFSET :offset;`,
		// Adding cfg.PictureStorage.URLDownload to sql here, to get urls in one query (we dont need to iterate and calculate URL for each record now)
		// another option is to create internal struct and iterate over query result and convert it to the public Referral.
		cfg.PictureStorage.URLDownload)
	params := map[string]interface{}{
		"user_id": id,
		"limit":   limit,
		"offset":  offset,
	}
	if err := u.db.PrepareExecuteTyped(sql, params, &queryResult); err != nil {
		return nil, errors.Wrap(err, "failed to get T1 referrals")
	}

	return queryResult, nil
}
