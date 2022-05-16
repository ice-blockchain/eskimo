// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
)

// GetTier1Referrals refers to the USERS space, so placing it to the users package.
func (u *users) GetTier1Referrals(ctx context.Context, id UserID, limit, offset uint64) ([]*Referral, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "failed to get referrals because of context failed")
	}

	var queryResult []*Referral
	// Referral = subject, referee = actor (main user).
	sql := fmt.Sprintf(`SELECT referrals.ID, referrals.username, referrals.phone_number, '%v/'||referrals.profile_picture_name AS profile_picture_url,
POSITION(referrals.phone_number_hash,referees.agenda_phone_number_hashes) > 0 as from_agenda FROM USERS referrals
INNER JOIN USERS referees ON referrals.referred_by = referees.ID
WHERE referrals.referred_by = :user_id ORDER BY from_agenda DESC, referrals.created_at DESC LIMIT :limit OFFSET :offset`,
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

func (u *users) GetReferredByUserCount(ctx context.Context, id UserID, days uint64, tier referralsLevel) (uint64, error) {
	if ctx.Err() != nil {
		return 0, errors.Wrapf(ctx.Err(), "failed to get tier %v user count because context failed", tier)
	}

	var sql string

	switch {
	case tier == "T1":
		sql = `SELECT COUNT(*) FROM users WHERE referred_by = :user_id AND created_at < :date`
	case tier == "T2":
		sql = `SELECT COUNT(*) FROM users WHERE referred_by IN (SELECT id FROM USERS WHERE referred_by = :user_id AND created_at < :date)`
	default:
		return 0, errors.Errorf("unknown tier level %v", tier)
	}

	timeLimit := time.Now().AddDate(0, 0, int(days)*-1).UTC().UnixNano()
	params := map[string]interface{}{"user_id": id, "date": timeLimit}

	var results []struct {
		Count uint64
	}

	if err := u.db.PrepareExecuteTyped(sql, params, &results); err != nil {
		return 0, errors.Wrap(err, "failed to get referred_by user count")
	}

	return results[0].Count, nil
}
