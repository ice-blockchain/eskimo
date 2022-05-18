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

func (u *users) GetReferralAcquisitionHistory(ctx context.Context, id UserID, days uint64) ([]*ReferralAcquisition, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "failed to get acquisition history because context failed")
	}

	nSecToDays := time.Hour.Nanoseconds() * 24 //nolint:gomnd //Nanoseconds in day
	//today := time.Now().UTC().Unix() / 86400   //nolint:gomnd //This is number of seconds in day
	today := time.Now().UTC().UnixNano()
	daysRange := u.getDaysRange(days)

	resT1, err := u.getT1Stats(id, nSecToDays, today, daysRange)
	if err != nil {
		return nil, errors.Wrap(err, "error getting t1 referral acquisition history")
	}

	resT2, err := u.getT2Stats(id, nSecToDays, today, daysRange)
	if err != nil {
		return nil, errors.Wrap(err, "error getting t2 referral acquisition history")
	}

	var result []*ReferralAcquisition

	for i := 0; i < len(resT1); i++ {
		tmp := new(ReferralAcquisition)
		tmp.T1 = resT1[i].Count
		tmp.T2 = resT2[i].Count
		tmp.Date = time.Unix(resT1[i].Date/time.Second.Nanoseconds(), 0).Round(time.Hour * 24) //nolint:gomnd //Round by day

		result = append(result, tmp)
	}

	return result, nil
}

func (u *users) getT1Stats(id UserID, nSecToDays, today int64, daysRange string) ([]*referralAcquisition, error) {
	/*
		sql := fmt.Sprintf("SELECT COUNT(*), created_at FROM USERS WHERE referred_by = :user_id "+
			"AND -1*(created_at/:nsec_to_days-:today) IN (%v) "+
			"GROUP BY (created_at/:nsec_to_days-:today) ORDER BY created_at DESC", daysRange)
	*/

	sql := fmt.Sprintf("SELECT COUNT(*), created_at FROM USERS WHERE referred_by = :user_id "+
		"AND -1*((created_at-:today)/:nsec_to_days) IN (%v) "+
		"GROUP BY (created_at-:today)/:nsec_to_days ORDER BY created_at DESC", daysRange)

	params := map[string]interface{}{
		"user_id":      id,
		"nsec_to_days": nSecToDays,
		"today":        today,
	}

	var results []*referralAcquisition

	if err := u.db.PrepareExecuteTyped(sql, params, &results); err != nil {
		return nil, errors.Wrap(err, "failed to get referred_by user count")
	}

	return results, nil
}

func (u *users) getT2Stats(id UserID, nSecToDays, today int64, daysRange string) ([]*referralAcquisition, error) {
	/*
		sql1 := fmt.Sprintf("SELECT id FROM USERS WHERE REFERRED_BY = :user_id AND -1*(created_at/:nsec_to_days-:today) IN (%v)", daysRange)

		sql := fmt.Sprintf("SELECT COUNT(*), created_at FROM users WHERE referred_by IN (%v) "+
			"AND -1*(created_at/:nsec_to_days-:today) IN (%v) "+
			"GROUP BY (created_at/:nsec_to_days-:today) ORDER BY created_at DESC", sql1, daysRange)
	*/

	sql := fmt.Sprintf("SELECT COUNT(*), created_at FROM users WHERE referred_by IN "+
		"(SELECT id FROM USERS WHERE REFERRED_BY = :user_id AND -1*((created_at-:today)/:nsec_to_days) IN (%v)) "+
		"AND -1*((created_at-:today)/:nsec_to_days) IN (%v) "+
		"GROUP BY (created_at-:today)/:nsec_to_days ORDER BY created_at DESC", daysRange, daysRange)

	params := map[string]interface{}{
		"user_id":      id,
		"nsec_to_days": nSecToDays,
		"today":        today,
	}

	var results []*referralAcquisition

	if err := u.db.PrepareExecuteTyped(sql, params, &results); err != nil {
		return nil, errors.Wrap(err, "failed to get referred_by user count")
	}

	return results, nil
}

func (u *users) getDaysRange(days uint64) string {
	var out string
	for i := 0; i < int(days); i++ {
		out += fmt.Sprintf("%v", i)
		if i != int(days)-1 {
			out += ","
		}
	}

	return out
}
