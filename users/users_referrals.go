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

//nolint:funlen // Long SQL
func (u *users) GetReferralAcquisitionHistory(ctx context.Context, id UserID, reqDays uint64) ([]*ReferralAcquisition, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "failed to get acquisition history because context failed")
	}

	days := time.Duration(reqDays)
	now := time.Now().UTC()
	nowNanos := now.UnixNano()
	nanosSinceMidnight := time.Duration(now.Nanosecond()) +
		time.Duration(now.Second())*time.Second +
		time.Duration(now.Minute())*time.Minute +
		time.Duration(now.Hour())*time.Hour
	pastNanos := now.Add(-days * 24 * time.Hour).Add(-nanosSinceMidnight).UnixNano()

	sql := `SELECT * FROM (WITH RECURSIVE referrals AS (
		SELECT  id,
		(:nowNanos - created_at) / 86400000000000 AS past_day,
		1                                                   AS t1,
		0                                                   AS t2,
		1                                                   AS tier
	FROM users
	WHERE 1 = 1
	AND referred_by = :userID
	AND created_at >= :pastNanos
	AND created_at <= :nowNanos

	UNION ALL

	SELECT i.id,
		(:nowNanos - i.created_at) / 86400000000000 AS past_day,
		0                                                     AS t1,
		1                                                     AS t2,
		tier + 1                                              AS tier
	FROM referrals
	JOIN users i
	ON referrals.id = i.referred_by
	AND i.created_at >= :pastNanos
	AND i.created_at <= :nowNanos
	WHERE tier < 3
	)
	SELECT
		TOTAL(referrals.t1) AS t1_referrals_acquired,
		TOTAL(referrals.t2) AS t2_referrals_acquired,
		days.day AS past_day
	FROM days
		LEFT JOIN referrals
			ON days.day = referrals.past_day	
    WHERE days.day <= :days
	GROUP BY days.day
	ORDER BY days.day)`

	params := map[string]interface{}{
		"userID":    id,
		"nowNanos":  nowNanos,
		"pastNanos": pastNanos,
		"days":      reqDays,
	}

	var resultFromQuery []*referralAcquisition

	if err := u.db.PrepareExecuteTyped(sql, params, &resultFromQuery); err != nil {
		return nil, errors.Wrap(err, "failed to get referred_by user count")
	}

	result := make([]*ReferralAcquisition, 0, reqDays+1)

	for i, row := range resultFromQuery {
		tmp := new(ReferralAcquisition)
		var date time.Time

		if i != 0 {
			date = now.AddDate(0, 0, int(-row.PastDay+1)).Add(-nanosSinceMidnight - 1)
		} else {
			date = now
		}

		tmp.Date = date
		tmp.T1 = uint64(row.CountT1)
		tmp.T2 = uint64(row.CountT2)

		result = append(result, tmp)
	}

	return result, nil
}
