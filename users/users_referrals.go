// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"
	"strconv"
	stdlibtime "time"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:funlen // It has a long SQL, it's better to be within the same method.
func (r *repository) GetReferrals(ctx context.Context, userID, referralType string, limit, offset uint64) (*Referrals, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "failed to get referrals because of context failed")
	}
	var referralTypeJoin string
	switch referralType {
	case Tier1Referrals:
		referralTypeJoin = `
			 JOIN USERS referrals
					ON referrals.referred_by = u.ID
					and referrals.id != u.id`
	case Tier2Referrals:
		referralTypeJoin = `
			 JOIN USERS t1
                	ON t1.referred_by = u.ID
					and t1.id != u.id
						JOIN USERS referrals
							ON referrals.referred_by = t1.ID
							and referrals.id != t1.id`
	case ContactsReferrals:
		referralTypeJoin = `
			JOIN USERS referrals
					ON POSITION(referrals.phone_number_hash, u.agenda_phone_number_hashes) > 0
					and referrals.id != u.id`
	default:
		log.Panic(errors.Errorf("referral type: '%v' not supported", referralType))
	}
	sql := fmt.Sprintf(`
		SELECT  0 																					   AS last_mining_started_at, 
				0 																					   AS last_ping_at,
				CAST(SUM(1) AS STRING) 																   AS total,
			    CAST(SUM(CASE 
							WHEN (:nowNanos - referrals.last_mining_started_at) < 86400000000000 
								THEN 1 
							ELSE 0 
				         END) AS STRING) 	 														   AS active,	 
				'' 																					   AS first_name, 
				'' 																					   AS last_name, 
				'' 																					   AS phone_number_, 
				''																					   AS profile_picture_url, 
				'' 																					   AS country,
				'' 																					   AS city
		FROM USERS u
				%[2]v
		WHERE u.id = :userId

		UNION ALL

		SELECT X.last_mining_started_at,
			   X.last_ping_at,
			   X.id,
			   X.username,
			   X.first_name,
			   X.last_name,
			   X.phone_number_,
			   X.profile_picture_url,
			   X.country,
			   X.city
		FROM (SELECT  
				referrals.last_mining_started_at                                                       AS last_mining_started_at,
				(CASE
					WHEN u.id = referrals.referred_by
						THEN referrals.last_ping_at
					ELSE :nowNanos
				 END)                                                                                  AS last_ping_at,
				referrals.ID                                                                           AS id,
				referrals.username                                                                     AS username,
				referrals.first_name                                                                   AS first_name,
				referrals.last_name                                                                    AS last_name,
				(CASE
					WHEN POSITION(referrals.phone_number_hash, u.agenda_phone_number_hashes) > 0
						THEN referrals.phone_number
					ELSE ''
				 END)                                                                                   AS phone_number_,
				'%[1]v/' || referrals.profile_picture_name                                              AS profile_picture_url,
				referrals.country                                                                       AS country,
				referrals.city                                                                       	AS city,
				referrals.created_at                                                                    AS created_at
		FROM USERS u
				%[2]v
		WHERE u.id = :userId
		ORDER BY (phone_number_ != '' AND phone_number_ != null) DESC,
				 referrals.created_at DESC
		LIMIT %[3]v OFFSET :offset) X`, cfg.PictureStorage.URLDownload, referralTypeJoin, limit)
	params := map[string]interface{}{
		"userId":   userID,
		"nowNanos": time.Now(),
		"offset":   offset,
	}
	var result []*Referral
	if err := r.db.PrepareExecuteTyped(sql, params, &result); err != nil {
		return nil, errors.Wrapf(err, "failed to get referrals for userID:%v,referralType%v,limit:%v,offset:%v", userID, referralType, limit, offset)
	}
	var err error
	var total, active uint64
	if result[0].ID != "" {
		//nolint:gomnd // Not a magic number.
		total, err = strconv.ParseUint(result[0].ID, 10, 64)
		log.Panic(err)
	}
	if result[0].Username != "" {
		//nolint:gomnd // Not a magic number.
		active, err = strconv.ParseUint(result[0].Username, 10, 64)
		log.Panic(err)
	}

	return &Referrals{
		Total:     total,
		Active:    active,
		Referrals: result[1:],
	}, nil
}

//nolint:funlen // It has a long SQL and specific time handling, it's better to be within the same method.
func (r *repository) GetReferralAcquisitionHistory(ctx context.Context, userID string, daysNumber uint64) ([]*ReferralAcquisition, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "failed to get acquisition history because context failed")
	}

	days := stdlibtime.Duration(daysNumber)
	now := time.Now()
	nowNanos := now.UnixNano()
	nanosSinceMidnight := stdlibtime.Duration(now.Nanosecond()) +
		stdlibtime.Duration(now.Second())*stdlibtime.Second +
		stdlibtime.Duration(now.Minute())*stdlibtime.Minute +
		stdlibtime.Duration(now.Hour())*stdlibtime.Hour
	pastNanos := stdlibtime.Unix(0, nowNanos).UTC().Add(-days * 24 * stdlibtime.Hour).Add(-nanosSinceMidnight).UnixNano()

	sql := `
SELECT * 
FROM (		
		WITH RECURSIVE referrals AS 
		(
			SELECT  id,
					(:nowNanos - created_at) / 86400000000000 AS past_day,
					1                                         AS t1,
					0                                         AS t2,
					1                                         AS tier
			FROM users
			WHERE 1 = 1
				AND referred_by = :userId
				AND id != :userId
				AND created_at >= :pastNanos
				AND created_at <= :nowNanos
		
			UNION ALL
		
			SELECT  i.id,
					(:nowNanos - i.created_at) / 86400000000000 AS past_day,
					0                                           AS t1,
					1                                           AS t2,
					tier + 1                                    AS tier
			FROM referrals
					JOIN users i
						ON referrals.id = i.referred_by
						AND referrals.id != i.id
						AND i.created_at >= :pastNanos
						AND i.created_at <= :nowNanos
			WHERE tier < 3
		)
		SELECT
			CAST(TOTAL(referrals.t1) AS INT) AS t1_referrals_acquired,
			CAST(TOTAL(referrals.t2) AS INT) AS t2_referrals_acquired,
			days.day AS past_day
		FROM days
			LEFT JOIN referrals
				ON days.day = referrals.past_day	
		WHERE days.day <= :days
		GROUP BY days.day
		ORDER BY days.day
     )`
	params := map[string]interface{}{
		"userId":    userID,
		"nowNanos":  nowNanos,
		"pastNanos": pastNanos,
		"days":      daysNumber,
	}
	var resultFromQuery []*struct {
		CountT1 uint64
		CountT2 uint64
		PastDay uint64
	}
	if err := r.db.PrepareExecuteTyped(sql, params, &resultFromQuery); err != nil {
		return nil, errors.Wrapf(err, "failed to select ReferralAcquisition history for userID:%v,days:%v", userID, daysNumber)
	}

	result := make([]*ReferralAcquisition, 0, daysNumber+1)
	for i, row := range resultFromQuery {
		tmp := new(ReferralAcquisition)
		var date *time.Time

		if i != 0 {
			date = time.New(stdlibtime.Unix(0, nowNanos).UTC().AddDate(0, 0, int(-row.PastDay+1)).Add(-nanosSinceMidnight - 1))
		} else {
			date = now
		}

		tmp.Date = date
		tmp.T1 = row.CountT1
		tmp.T2 = row.CountT2

		result = append(result, tmp)
	}

	return result, nil
}
