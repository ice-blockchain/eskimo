// SPDX-License-Identifier: ice License 1.0

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
func (r *repository) GetReferrals(ctx context.Context, userID string, referralType ReferralType, limit, offset uint64) (*Referrals, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "failed to get referrals because of context failed")
	}
	before2 := time.Now()
	defer func() {
		if elapsed := stdlibtime.Since(*before2.Time); elapsed > 100*stdlibtime.Millisecond {
			log.Info(fmt.Sprintf("[response]GetReferrals(%v) took: %v", referralType, elapsed))
		}
	}()
	totalAndActiveColumns := `  CAST(SUM(1) AS STRING) 																   			AS total,
								CAST(SUM(CASE 
											WHEN COALESCE(referrals.last_mining_ended_at,0) > :nowNanos 
												THEN 1 
											ELSE 0 
										 END) AS STRING) 	 														   			AS active,`
	var referralTypeJoin, referralTypeJoinSumAgg string
	switch referralType {
	case Tier1Referrals:
		referralTypeJoin = `
			 JOIN USERS referrals
					ON (referrals.referred_by = u.id OR u.referred_by = referrals.id)
                   AND referrals.username != referrals.id
                   AND referrals.referred_by != referrals.id`
	case Tier2Referrals:
		referralTypeJoin = `
			 JOIN USERS t1
                	ON t1.referred_by = u.ID
					AND t1.id != u.id
                   	AND t1.username != t1.id
                    AND t1.referred_by != t1.id
						JOIN USERS referrals
							ON referrals.referred_by = t1.ID
							AND referrals.id != t1.id
                   			AND referrals.username != referrals.id
						    AND referrals.referred_by != referrals.id`
	case ContactsReferrals:
		referralTypeJoin = `
			JOIN USERS referrals
					ON NULLIF(referrals.phone_number_hash,'') IS NOT NULL
					AND POSITION(referrals.phone_number_hash, u.agenda_phone_number_hashes) > 0
                    AND referrals.username != referrals.id
					AND referrals.referred_by != referrals.id
					AND u.id != referrals.id`
		totalAndActiveColumns = `'0' AS total,
								 '0' AS active,`
	default:
		log.Panic(errors.Errorf("referral type: '%v' not supported", referralType))
	}
	if referralType != ContactsReferrals {
		referralTypeJoinSumAgg = referralTypeJoin
	}
	sql := fmt.Sprintf(`
		SELECT  0 																					   AS last_mining_ended_at, 
				0 																					   AS last_ping_cooldown_ended_at,
				'' 																					   AS phone_number_,
				'' 																					   AS email,
				%[4]v	 
				''																					   AS profile_picture_url, 
				''																					   AS country, 
				''																					   AS city, 
				''																					   AS referral_type 
		FROM USERS u
				%[5]v
		WHERE u.id = :userId

		UNION ALL

		SELECT X.last_mining_ended_at,
			   X.last_ping_cooldown_ended_at,
			   X.phone_number_,
			   '' AS email,
			   X.id,
			   X.username,
			   X.profile_picture_url,
			   X.country,
			   '' AS city,
			   :referralType AS referral_type
		FROM (SELECT  
				COALESCE(referrals.last_mining_ended_at,1)                                             AS last_mining_ended_at,
				(CASE
					WHEN u.id = referrals.referred_by OR u.referred_by = referrals.id
						THEN (CASE 
									WHEN COALESCE(referrals.last_mining_ended_at,0) < :nowNanos 
									    THEN COALESCE(referrals.last_ping_cooldown_ended_at,1) 
								   	ELSE :nowNanos 
							  END)
					ELSE null
				 END)                                                                                  AS last_ping_cooldown_ended_at,
				referrals.ID                                                                           AS id,
				referrals.username                                                                     AS username,
				referrals.country                                                                      AS country,
				(CASE
					WHEN NULLIF(referrals.phone_number_hash,'') IS NOT NULL AND POSITION(referrals.phone_number_hash, u.agenda_phone_number_hashes) > 0
						THEN referrals.phone_number
					ELSE ''
				 END)                                                                                   AS phone_number_,
				%[1]v                                              										AS profile_picture_url,
				referrals.created_at                                                                    AS created_at
				FROM USERS u
						%[2]v
				WHERE u.id = :userId
				ORDER BY (phone_number_ != '' AND phone_number_ != null) DESC,
						 referrals.created_at DESC
				LIMIT %[3]v OFFSET :offset
			 ) X`, r.pictureClient.SQLAliasDownloadURL(`referrals.profile_picture_name`), referralTypeJoin, limit, totalAndActiveColumns, referralTypeJoinSumAgg) //nolint:lll // .
	params := map[string]any{
		"userId":       userID,
		"referralType": referralType,
		"offset":       offset,
		"nowNanos":     time.Now(),
	}
	var result []*MinimalUserProfile
	if err := r.db.PrepareExecuteTyped(sql, params, &result); err != nil {
		return nil, errors.Wrapf(err, "failed to get referrals for userID:%v,referralType:%v,limit:%v,offset:%v", userID, referralType, limit, offset)
	}
	if len(result) == 0 {
		return &Referrals{
			UserCount: UserCount{
				Total:  0,
				Active: 0,
			},
			Referrals: make([]*MinimalUserProfile, 0),
		}, nil
	}
	var err error
	var total, active uint64
	if result[0].ID != "" {
		total, err = strconv.ParseUint(result[0].ID, 10, 64)
		log.Panic(err)
	}
	if result[0].Username != "" {
		active, err = strconv.ParseUint(result[0].Username, 10, 64)
		log.Panic(err)
	}

	return &Referrals{
		UserCount: UserCount{
			Total:  total,
			Active: active,
		},
		Referrals: result[1:],
	}, nil
}

//nolint:funlen // It has a long SQL and specific time handling, it's better to be within the same method.
func (r *repository) GetReferralAcquisitionHistory(ctx context.Context, userID string, daysNumber uint64) ([]*ReferralAcquisition, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "failed to get acquisition history because context failed")
	}

	before2 := time.Now()
	defer func() {
		if elapsed := stdlibtime.Since(*before2.Time); elapsed > 100*stdlibtime.Millisecond {
			log.Info(fmt.Sprintf("[response]GetReferralAcquisitionHistory took: %v", elapsed))
		}
	}()
	days := stdlibtime.Duration(daysNumber)
	now := time.Now()
	nowNanos := now.UnixNano()
	nsSinceMidnight := NanosSinceMidnight(now)
	pastNanos := stdlibtime.Unix(0, nowNanos).UTC().Add(-days * 24 * stdlibtime.Hour).Add(-nsSinceMidnight).UnixNano()
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
				AND username != id
				AND referred_by != id
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
						AND i.referred_by != i.id
						AND i.username != i.id
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
		WHERE days.day < :days
		GROUP BY days.day
		ORDER BY days.day
     )`
	params := map[string]any{
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

	result := make([]*ReferralAcquisition, 0, daysNumber)
	for i, row := range resultFromQuery {
		tmp := new(ReferralAcquisition)
		var date *time.Time

		if i != 0 {
			date = time.New(now.AddDate(0, 0, int(-row.PastDay+1)).Add(-nsSinceMidnight - 1))
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
