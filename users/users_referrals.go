// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	stdlibtime "time"

	"github.com/pkg/errors"

	storage "github.com/ice-blockchain/wintr/connectors/storage/v2"
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
	totalAndActiveColumns := `  CAST(COALESCE(SUM(1), 0) AS text) 												   	AS id,
								CAST(COALESCE(SUM(CASE 
											WHEN COALESCE(referrals.last_mining_ended_at, to_timestamp(0)) > $4 
												THEN 1 
											ELSE 0 
										 END), 0) AS text) 	 														AS username,`
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
		if true {
			return &Referrals{
				Referrals: make([]*MinimalUserProfile, 0),
			}, nil
		}
		referralTypeJoin = `
			JOIN USERS referrals
					ON NULLIF(referrals.phone_number_hash,'') IS NOT NULL
					AND POSITION(referrals.phone_number_hash IN u.agenda_phone_number_hashes) > 0
                    AND referrals.username != referrals.id
					AND referrals.referred_by != referrals.id
					AND u.id != referrals.id`
		totalAndActiveColumns = `'0' 																   				AS id,
								 '0' 																   				AS username,`
	default:
		log.Panic(errors.Errorf("referral type: '%v' not supported", referralType))
	}
	if referralType != ContactsReferrals {
		referralTypeJoinSumAgg = referralTypeJoin
	}
	sql := fmt.Sprintf(`
		SELECT  to_timestamp(0)																		   				AS active, 
				to_timestamp(0)																		   				AS pinged,
				'' 																					   				AS phone_number,
				'' 																					   				AS email,
				%[4]v	 
				''																					   				AS profile_picture_url, 
				''																					   				AS country, 
				''																					   				AS city, 
				''																					   				AS referral_type 
		FROM USERS u
				%[5]v
		WHERE u.id = $1

		UNION ALL

		SELECT X.last_mining_ended_at  		 			 											   				AS active,
			   X.last_ping_cooldown_ended_at  			 											   				AS pinged,
			   X.phone_number_ 							 											   				AS phone_number,
			   '' AS email,
			   X.id,
			   X.username,
			   X.profile_picture_url 					 											   				AS profile_picture_name,
			   X.country,
			   '' AS city,
			   $2 AS referral_type
		FROM (SELECT  
				COALESCE(referrals.last_mining_ended_at, to_timestamp(0))              				   				AS last_mining_ended_at,
				(CASE
					WHEN u.id = referrals.referred_by OR u.referred_by = referrals.id
						THEN (CASE 
									WHEN COALESCE(referrals.last_mining_ended_at,to_timestamp(0)) < $4 
									    THEN COALESCE(referrals.last_ping_cooldown_ended_at,to_timestamp(0)) 
								   	ELSE $4 
							  END)
					ELSE null
				 END)                                                                                  				AS last_ping_cooldown_ended_at,
				referrals.ID                                                                           				AS id,
				referrals.username                                                                     				AS username,
				referrals.country                                                                      				AS country,
				(CASE
					WHEN NULLIF(referrals.phone_number_hash,'') IS NOT NULL AND POSITION(referrals.phone_number_hash IN u.agenda_phone_number_hashes) > 0
						THEN referrals.phone_number
					ELSE ''
				 END)                                                                                  				AS phone_number_,
				%[1]v                                              									   				AS profile_picture_url,
				referrals.created_at                                                                   				AS created_at
				FROM USERS u
						%[2]v
				WHERE u.id = $1
				ORDER BY ((CASE WHEN NULLIF(referrals.phone_number_hash,'') IS NOT NULL AND POSITION(referrals.phone_number_hash IN u.agenda_phone_number_hashes) > 0
								THEN referrals.phone_number
								ELSE ''
				 		   END) != ''
						  AND 
						  (CASE WHEN NULLIF(referrals.phone_number_hash,'') IS NOT NULL AND POSITION(referrals.phone_number_hash IN u.agenda_phone_number_hashes) > 0
						  		THEN referrals.phone_number
					  			ELSE ''
					 	   END) != null) DESC,
						 referrals.created_at DESC
				LIMIT %[3]v OFFSET $3
			 ) X`, r.pictureClient.SQLAliasDownloadURL(`referrals.profile_picture_name`), referralTypeJoin, limit, totalAndActiveColumns, referralTypeJoinSumAgg) //nolint:lll // .
	args := []any{userID, referralType, offset, time.Now().Time}
	result, err := storage.Select[MinimalUserProfile](ctx, r.db, sql, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to select for all t1 referrals of userID:%v + their new random referralID", userID)
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

//nolint:funlen // Long SQL with field list.
func (r *repository) GetReferralAcquisitionHistory(ctx context.Context, userID string) ([]*ReferralAcquisition, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "failed to get acquisition history because context failed")
	}

	before2 := time.Now()
	defer func() {
		if elapsed := stdlibtime.Since(*before2.Time); elapsed > 100*stdlibtime.Millisecond {
			log.Info(fmt.Sprintf("[response]GetReferralAcquisitionHistory took: %v", elapsed))
		}
	}()
	now := time.Now()
	nowMidnight := time.New(time.Now().In(stdlibtime.UTC).Truncate(hoursInOneDay * stdlibtime.Hour))
	sql := `
	SELECT 
		date,
		t1_today,
		t1_today_minus_1,
		t1_today_minus_2,
		t1_today_minus_3,
		t1_today_minus_4,
		t2_today,
		t2_today_minus_1,
		t2_today_minus_2,
		t2_today_minus_3,
		t2_today_minus_4
    from referral_acquisition_history
		where user_id = $1
`
	type resultFromQuery struct {
		Date          *time.Time
		T1Today       int64 `db:"t1_today"`
		T2Today       int64 `db:"t2_today"`
		T1TodayMinus1 int64 `db:"t1_today_minus_1"`
		T2TodayMinus1 int64 `db:"t2_today_minus_1"`
		T1TodayMinus2 int64 `db:"t1_today_minus_2"`
		T2TodayMinus2 int64 `db:"t2_today_minus_2"`
		T1TodayMinus3 int64 `db:"t1_today_minus_3"`
		T2TodayMinus3 int64 `db:"t2_today_minus_3"`
		T1TodayMinus4 int64 `db:"t1_today_minus_4"`
		T2TodayMinus4 int64 `db:"t2_today_minus_4"`
	}
	res, err := storage.Get[resultFromQuery](ctx, r.db, sql, userID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to select ReferralAcquisition history for userID:%v", userID)
	}
	elapsedDaysSinceLastRefCountsUpdate := int(nowMidnight.Sub(*res.Date.Time).Nanoseconds() / int64(hoursInOneDay*stdlibtime.Hour))
	if elapsedDaysSinceLastRefCountsUpdate > maxDaysReferralsHistory {
		elapsedDaysSinceLastRefCountsUpdate = maxDaysReferralsHistory
	}
	result := make([]*ReferralAcquisition, maxDaysReferralsHistory) //nolint:makezero // We're know size for sure.
	orderOfDaysT1 := []int64{res.T1Today, res.T1TodayMinus1, res.T1TodayMinus2, res.T1TodayMinus3, res.T1TodayMinus4}
	orderOfDaysT2 := []int64{res.T2Today, res.T2TodayMinus1, res.T2TodayMinus2, res.T2TodayMinus3, res.T2TodayMinus4}
	for ind := 0; ind < elapsedDaysSinceLastRefCountsUpdate; ind++ {
		var date *time.Time
		if ind != 0 {
			date = time.New(now.AddDate(0, 0, -ind))
		} else {
			date = now
		}
		result[ind] = &ReferralAcquisition{
			Date: date,
			T1:   uint64(orderOfDaysT1[0]),
			T2:   uint64(orderOfDaysT2[0]),
		}
	}
	for day := elapsedDaysSinceLastRefCountsUpdate; day < maxDaysReferralsHistory; day++ {
		date := time.New(now.AddDate(0, 0, -day))
		result[day] = &ReferralAcquisition{
			Date: date,
			T1:   uint64(orderOfDaysT1[day-elapsedDaysSinceLastRefCountsUpdate]),
			T2:   uint64(orderOfDaysT2[day-elapsedDaysSinceLastRefCountsUpdate]),
		}
	}

	return result, nil
}

func (r *repository) updateReferralCount(ctx context.Context, us *UserSnapshot) error {
	if us.Before == nil {
		return errors.Wrapf(r.insertReferralAcquisitionHistory(ctx, us.ID), "failed to insert referral counts for %#v", us)
	}
	if us.ReferredBy == us.ID || us.Before == nil || us.User == nil || us.Before.ReferredBy == us.ReferredBy {
		return nil
	}

	return errors.Wrapf(r.incrementReferralCount(ctx, us.ReferredBy), "failed to increment referrals count for userID:%v", us.ID)
}

func (r *repository) insertReferralAcquisitionHistory(ctx context.Context, userID UserID) error {
	sql := `
		INSERT INTO referral_acquisition_history(user_id, date)
			VALUES ($1, $2)
		ON CONFLICT (user_id) DO NOTHING`
	now := time.Now().In(stdlibtime.UTC).Truncate(hoursInOneDay * stdlibtime.Hour)
	_, err := storage.Exec(ctx, r.db, sql, userID, now)

	return errors.Wrapf(err, "failed to insert initial referral_acquisition_history for userID %v", userID)
}

//nolint:gocritic,revive // Struct is private, so we return values from it.
func (r *repository) getCurrentReferralCount(ctx context.Context, userID UserID) (t1, t2 int64, date *time.Time, err error) {
	type refCount struct {
		Date *time.Time
		T1   int64
		T2   int64
	}
	sql := `
		WITH t2 AS (
			SELECT id FROM (SELECT referred_by AS id FROM users WHERE id = $1 AND referred_by != id
            UNION SELECT NULL as id) tmp LIMIT 1
		) SELECT t1_today AS t1,
			   COALESCE((SELECT t2_today from referral_acquisition_history where user_id = t2.id),0) AS t2,
			   date
		FROM referral_acquisition_history, t2 WHERE user_id = $1
	`
	count, err := storage.Get[refCount](ctx, r.db, sql, userID)
	if err != nil {
		return 0, 0, nil, errors.Wrapf(err, "failed to read current referral count for userID:%v", userID)
	}

	return count.T1, count.T2, count.Date, nil
}

// nolint:funlen // Long SQL.
func (r *repository) incrementReferralCount(ctx context.Context, userID UserID) error {
	nowMidnight := time.New(time.Now().In(stdlibtime.UTC).Truncate(hoursInOneDay * stdlibtime.Hour))
	t1, t2, storedDate, err := r.getCurrentReferralCount(ctx, userID)
	if err != nil {
		if storage.IsErr(err, storage.ErrNotFound) {
			if insErr := r.insertReferralAcquisitionHistory(ctx, userID); insErr != nil {
				return errors.Wrapf(insErr, "failed to update / insert referrals counts for userID %v", userID)
			}

			return r.incrementReferralCount(ctx, userID)
		}

		return errors.Wrapf(err, "failed to read current value")
	}
	if !nowMidnight.Equal(*storedDate.Time) {
		if err = r.shiftReferralAcquisitionDaysForUserID(ctx, userID, storedDate, nowMidnight); err != nil {
			return errors.Wrapf(err, "failed to shift dates for referral acquisition, from %v to %v (userID %v)", storedDate, nowMidnight, userID)
		}
	}
	sql := `
		WITH t2 AS (
			SELECT id FROM (SELECT referred_by AS id FROM users WHERE id = $1 AND referred_by != id
            UNION SELECT NULL as id) tmp LIMIT 1
		) UPDATE referral_acquisition_history
		SET
			t1_today = (CASE
				WHEN user_id = $1 THEN t1_today + 1
				ELSE t1_today END),
			t2_today = (CASE
			WHEN user_id = t2.id THEN t2_today +1
			ELSE t2_today END),
			DATE = $5
		FROM t2
		WHERE date = $2 AND ((user_id = $1 AND t1_today = $3)
  				     		or (user_id = t2.id AND t2_today = $4))`
	rowsUpdated, err := storage.Exec(ctx, r.db, sql, userID, storedDate.Time, t1, t2, nowMidnight.Time)
	if rowsUpdated == 0 || storage.IsErr(err, storage.ErrNotFound) {
		return r.incrementReferralCount(ctx, userID)
	}

	return errors.Wrapf(err, "failed to insert initial referral_acquisition_history for userID %v", userID)
}

func (r *repository) shiftReferralAcquisitionDaysForUserID(ctx context.Context, userID UserID, prevDate, now *time.Time) error {
	shiftDays := now.Sub(*prevDate.Time).Nanoseconds() / int64(hoursInOneDay*stdlibtime.Hour)
	if shiftDays == 0 {
		return nil
	}
	if shiftDays > maxDaysReferralsHistory {
		shiftDays = maxDaysReferralsHistory
	}
	sql := fmt.Sprintf(`
			WITH t2 AS (
			SELECT id FROM (SELECT referred_by AS id FROM users WHERE id = $2 AND referred_by != id
            UNION SELECT NULL as id) tmp LIMIT 1
			) UPDATE referral_acquisition_history SET
			   %v,
			   %v,
			   date = $3
			   FROM t2
			   WHERE ((user_id = $2 AND date = $1) OR user_id = t2.id)
			`,
		r.generateReferralsShiftDaysSQL(Tier1Referrals, shiftDays),
		r.generateReferralsShiftDaysSQL(Tier2Referrals, shiftDays),
	)
	rowsUpdated, err := storage.Exec(ctx, r.db, sql, prevDate.Time, userID, now.Time)
	if rowsUpdated == 0 || storage.IsErr(err, storage.ErrNotFound) {
		return nil // Already updated?
	}

	return errors.Wrapf(err, "failed to shift the dates of referrals acquisition for userID %v", userID)
}

func (*repository) generateReferralsShiftDaysSQL(refType ReferralType, daysLag int64) string {
	updateStatements := []string{}
	for currDay := 1; currDay <= maxDaysReferralsHistory-1; currDay++ {
		diff := int64(currDay) - daysLag
		targetField := fmt.Sprintf("%v_today_minus_%v", refType, diff)
		if diff <= 0 {
			targetField = fmt.Sprintf("%v_today", refType)
		}
		updateStatements = append(updateStatements, fmt.Sprintf("%v_today_minus_%v = %v", refType, currDay, targetField))
	}

	return strings.Join(updateStatements, ",\n")
}
