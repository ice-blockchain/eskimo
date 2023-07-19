// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"fmt"
	"math"
	"math/rand"
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
		referralTypeJoin = `
			JOIN USERS referrals
					ON NULLIF(referrals.phone_number_hash,'') IS NOT NULL
					AND referrals.id = ANY(u.agenda_contact_user_ids)
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
				%[3]v	 
				''																					   				AS profile_picture_name, 
				''																					   				AS country, 
				''																					   				AS city, 
				''																					   				AS referral_type 
		FROM USERS u
				%[4]v
		WHERE u.id = $1

		UNION ALL

		SELECT X.last_mining_ended_at  		 			 											   				AS active,
			   X.last_ping_cooldown_ended_at  			 											   				AS pinged,
			   X.phone_number_ 							 											   				AS phone_number,
			   '' AS email,
			   X.id,
			   X.username,
			   X.profile_picture_name 					 											   				AS profile_picture_name,
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
								   	ELSE referrals.last_mining_ended_at
							  END)
					ELSE null
				 END)                                                                                  				AS last_ping_cooldown_ended_at,
				referrals.ID                                                                           				AS id,
				referrals.username                                                                     				AS username,
				referrals.country                                                                      				AS country,
				(CASE
					WHEN NULLIF(referrals.phone_number_hash,'') IS NOT NULL AND referrals.id = ANY(u.agenda_contact_user_ids)
						THEN referrals.phone_number
					ELSE ''
				 END)                                                                                  				AS phone_number_,
				%[1]v                                              									   				AS profile_picture_name,
				referrals.created_at                                                                   				AS created_at
				FROM USERS u
						%[2]v
				WHERE u.id = $1
				ORDER BY ((CASE WHEN NULLIF(referrals.phone_number_hash,'') IS NOT NULL AND referrals.id = ANY(u.agenda_contact_user_ids)
								THEN referrals.phone_number
								ELSE ''
				 		   END) != ''
						  AND 
						  (CASE WHEN NULLIF(referrals.phone_number_hash,'') IS NOT NULL AND referrals.id = ANY(u.agenda_contact_user_ids)
						  		THEN referrals.phone_number
					  			ELSE ''
					 	   END) != null) DESC,
						 referrals.created_at DESC
				LIMIT $5 OFFSET $3
			 ) X`, r.pictureClient.SQLAliasDownloadURL(`referrals.profile_picture_name`), referralTypeJoin, totalAndActiveColumns, referralTypeJoinSumAgg) //nolint:lll // .
	args := []any{userID, referralType, offset, time.Now().Time, limit}
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

	now := time.Now()
	nowMidnight := time.New(time.Now().In(stdlibtime.UTC).Truncate(hoursInOneDay * stdlibtime.Hour))
	sql := `
		SELECT *
		from referral_acquisition_history
			where user_id = $1`
	type resultFromQuery struct {
		Date          *time.Time `db:"date"`
		UserID        UserID     `db:"user_id"`
		T1            int64      `db:"t1"`
		T2            int64      `db:"t2"`
		T1Today       int64      `db:"t1_today"`
		T2Today       int64      `db:"t2_today"`
		T1TodayMinus1 int64      `db:"t1_today_minus_1"`
		T2TodayMinus1 int64      `db:"t2_today_minus_1"`
		T1TodayMinus2 int64      `db:"t1_today_minus_2"`
		T2TodayMinus2 int64      `db:"t2_today_minus_2"`
		T1TodayMinus3 int64      `db:"t1_today_minus_3"`
		T2TodayMinus3 int64      `db:"t2_today_minus_3"`
		T1TodayMinus4 int64      `db:"t1_today_minus_4"`
		T2TodayMinus4 int64      `db:"t2_today_minus_4"`
	}
	res, err := storage.Get[resultFromQuery](ctx, r.db, sql, userID)
	if err != nil {
		if storage.IsErr(err, storage.ErrNotFound) {
			return []*ReferralAcquisition{}, nil
		}

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

func (r *repository) updateReferralCount(ctx context.Context, msgTimestamp stdlibtime.Time, us *UserSnapshot) error {
	var userID, referredBy string
	dayBetweenCreationAndDeletion := int16(-1)
	if us.Before != nil {
		userID = us.Before.ID
		referredBy = us.Before.ReferredBy
		dayBetweenCreationAndDeletion = int16(msgTimestamp.Sub(*us.Before.CreatedAt.Time) / (hoursInOneDay * stdlibtime.Hour))
	}

	if us.User != nil {
		if us.ReferredBy == us.ID || us.ReferredBy == "" || (us.Before != nil && us.Before.ReferredBy == us.ReferredBy) {
			return nil
		}
		userID = us.ID
		referredBy = us.ReferredBy
		dayBetweenCreationAndDeletion = -1
	}
	_, err := storage.Exec(ctx, r.db, `INSERT INTO processed_referrals(user_id, referred_by, processed_at, deleted) VALUES ($1, $2, $3, $4)`,
		userID, referredBy, msgTimestamp, us.User == nil)
	if storage.IsErr(err, storage.ErrDuplicate) {
		return nil
	} else if err != nil {
		return errors.Wrapf(err, "failed to verify uniqueness of user referral message")
	}

	return errors.Wrapf(r.incrementOrDecrementReferralCount(ctx, referredBy, dayBetweenCreationAndDeletion),
		"failed to update referrals count for userID:%v", userID)
}

//nolint:gocritic,revive // Struct is private, so we return values from it.
func (r *repository) getCurrentReferralCount(ctx context.Context, userID UserID) (t1, t2 int64, date, T0Date *time.Time, T0UserID UserID, err error) {
	type refCount struct {
		Date     *time.Time `db:"date"`
		T0Date   *time.Time `db:"t0_date"`
		T0UserID UserID     `db:"t0_user_id"`
		T1       int64
		T2       int64
	}
	sql := `
	WITH t0 AS (
		SELECT id FROM (SELECT referred_by AS id FROM users WHERE id = $1 AND referred_by != id
						UNION ALL SELECT NULL as id) tmp LIMIT 1
	) SELECT
		  COALESCE(refs.t1_today,0) AS t1,
		  COALESCE(t0_refs.t2_today,0) AS t2,
		  refs.date,
		  COALESCE(t0.id,'') as t0_user_id,
		  t0_refs.date as t0_date
	FROM t0
		LEFT JOIN referral_acquisition_history refs ON refs.user_id = $1
		LEFT JOIN referral_acquisition_history t0_refs ON t0_refs.user_id = t0.id
	`
	count, err := storage.Get[refCount](ctx, r.db, sql, userID)
	if err != nil {
		return 0, 0, nil, nil, "", errors.Wrapf(err, "failed to read current referral count for userID:%v", userID)
	}
	if count.Date == nil {
		return 0, count.T2, nil, count.T0Date, count.T0UserID, errors.Wrapf(storage.ErrNotFound, "failed to read current referral count for userID:%v", userID)
	}

	return count.T1, count.T2, count.Date, count.T0Date, count.T0UserID, nil
}

//nolint:funlen // Long SQL.
func (r *repository) incrementOrDecrementReferralCount(ctx context.Context, userID UserID, daysBetweenCreationAndDeletion int16) error {
	if ctx.Err() != nil {
		return errors.Wrapf(ctx.Err(), "ctx failed: ")
	}
	op := "+"
	opToday := "+"
	if daysBetweenCreationAndDeletion >= 0 {
		op = "-"
		opToday = "*" // Multiply by 1 = noop.
		if daysBetweenCreationAndDeletion == 0 {
			opToday = "-"
		}
	}
	nowMidnight := time.New(time.Now().In(stdlibtime.UTC).Truncate(hoursInOneDay * stdlibtime.Hour))
	t1, t2, storedDate, t0Date, t0UserID, err := r.getCurrentReferralCount(ctx, userID)
	if err != nil && !storage.IsErr(err, storage.ErrNotFound) {
		return errors.Wrapf(err, "failed to read current value")
	}
	if storedDate == nil {
		storedDate = nowMidnight
	}
	if t0Date == nil {
		t0Date = nowMidnight
	}
	shiftDays := int16(math.Min(float64(nowMidnight.Sub(*storedDate.Time).Nanoseconds()/int64(hoursInOneDay*stdlibtime.Hour)), maxDaysReferralsHistory))
	shiftDaysT0 := int16(math.Min(float64(nowMidnight.Sub(*t0Date.Time).Nanoseconds()/int64(hoursInOneDay*stdlibtime.Hour)), maxDaysReferralsHistory))

	shiftDatesFields := r.genShiftDaysSQLFields(shiftDays, shiftDaysT0, daysBetweenCreationAndDeletion)
	t0UpdateTrigger := ""
	if t0UserID != "" {
		t0UpdateTrigger = ",\n\t\t\t       ($6, $5, 1, 1, 1, 1)"
	}
	sql := fmt.Sprintf(`INSERT INTO referral_acquisition_history(user_id, date, t1, t1_today, t2, t2_today)
			VALUES ($1, $5, 1, 1, 0, 0)
			%[2]v
		ON CONFLICT (user_id) DO UPDATE 
		SET
			t1_today = (CASE
				WHEN referral_acquisition_history.user_id = $1 and referral_acquisition_history.date = $5 THEN GREATEST(referral_acquisition_history.t1_today %[4]v 1,0)
				WHEN referral_acquisition_history.user_id = $1 and referral_acquisition_history.date != $5 THEN GREATEST(0 %[4]v 1, 0)
				WHEN referral_acquisition_history.date = $5 THEN referral_acquisition_history.t1_today 
			    ELSE 0 END),
			t1 = (CASE
				WHEN referral_acquisition_history.user_id = $1 THEN GREATEST(referral_acquisition_history.t1 %[3]v 1,0)
				ELSE referral_acquisition_history.t1 END),
			t2_today = (CASE
			WHEN referral_acquisition_history.user_id = $6 AND referral_acquisition_history.date = $5 THEN GREATEST(referral_acquisition_history.t2_today %[4]v 1, 0)
			WHEN referral_acquisition_history.user_id = $6 AND referral_acquisition_history.date != $5 THEN GREATEST(0 %[4]v 1, 0)
			WHEN referral_acquisition_history.date = $5 THEN referral_acquisition_history.t2_today
		    ELSE 0 END),
			t2 = (CASE
			WHEN referral_acquisition_history.user_id = $6 THEN GREATEST(referral_acquisition_history.t2 %[3]v 1,0)
			ELSE referral_acquisition_history.t2 END),
			date = $5
		    %[1]v
		WHERE (referral_acquisition_history.date = $2 AND referral_acquisition_history.user_id = $1 AND referral_acquisition_history.t1_today = $3)
  		   OR (referral_acquisition_history.date = $7 AND referral_acquisition_history.user_id = $6 AND referral_acquisition_history.t2_today = $4)`,
		shiftDatesFields,
		t0UpdateTrigger,
		op,
		opToday,
	)
	rowsUpdated, err := storage.Exec(ctx, r.db, sql, userID, storedDate.Time, t1, t2, nowMidnight.Time, t0UserID, t0Date.Time)
	if rowsUpdated == 0 || storage.IsErr(err, storage.ErrNotFound) {
		return r.incrementOrDecrementReferralCount(ctx, userID, daysBetweenCreationAndDeletion)
	}

	return errors.Wrapf(err, "failed to increment referral counts for userID %v", userID)
}

func (r *repository) genShiftDaysSQLFields(shiftDays, t0ShiftDays, daysBetweenUserCreationAndDeletion int16) string {
	if shiftDays == 0 && t0ShiftDays == 0 && (daysBetweenUserCreationAndDeletion <= 0 || daysBetweenUserCreationAndDeletion >= maxDaysReferralsHistory) {
		return ""
	}

	return fmt.Sprintf(", %v,\n %v",
		r.generateReferralsShiftDaysSQL(Tier1Referrals, shiftDays, t0ShiftDays, daysBetweenUserCreationAndDeletion),
		r.generateReferralsShiftDaysSQL(Tier2Referrals, shiftDays, t0ShiftDays, daysBetweenUserCreationAndDeletion),
	)
}

func (r *repository) generateReferralsShiftDaysSQL(refType ReferralType, daysLag, t0daysLag, daysBetweenUserCreationAndDeletion int16) string {
	updateStatements := []string{}
	for currDay := int16(1); currDay <= maxDaysReferralsHistory-1; currDay++ {
		diff := currDay - daysLag
		diffT0 := currDay - t0daysLag
		decrementDueToUserDeletion := ""
		if currDay == daysBetweenUserCreationAndDeletion {
			decrementDueToUserDeletion = " - 1"
		}
		targetFieldT0 := r.findTargetField(refType, diffT0, t0daysLag, currDay, decrementDueToUserDeletion)
		targetField := r.findTargetField(refType, diff, daysLag, currDay, decrementDueToUserDeletion)
		field := ""
		switch {
		case targetFieldT0 != "" && targetField != "":
			field = fmt.Sprintf(
				"%[1]v_today_minus_%[2]v = (CASE WHEN referral_acquisition_history.user_id = $1 THEN GREATEST(%[3]v %[4]v,0) ELSE GREATEST(%[5]v %[4]v,0) END)",
				refType, currDay, targetField, decrementDueToUserDeletion, targetFieldT0,
			)
		case targetFieldT0 != "":
			field = fmt.Sprintf("%v_today_minus_%v = GREATEST(%v %v,0)", refType, currDay, targetFieldT0, decrementDueToUserDeletion)
		case targetField != "":
			field = fmt.Sprintf("%v_today_minus_%v = GREATEST(%v %v,0)", refType, currDay, targetField, decrementDueToUserDeletion)
		}
		if field == "" {
			continue
		}
		updateStatements = append(updateStatements, field)
	}

	return strings.Join(updateStatements, ",\n")
}

func (*repository) findTargetField(refType ReferralType, diff, daysLag, currDay int16, decrementDueToUserDeletion string) string {
	targetField := fmt.Sprintf("referral_acquisition_history.%v_today_minus_%v", refType, diff)
	if diff == 0 {
		targetField = fmt.Sprintf("referral_acquisition_history.%v_today", refType)
	} else if diff < 0 {
		targetField = "0"
	}
	if daysLag == 0 && decrementDueToUserDeletion != "" { // If we dont need to shift, but to sub 1, due to user deletion, subtract it from the same field.
		targetField = fmt.Sprintf("referral_acquisition_history.%v_today_minus_%v", refType, currDay)
	} else if daysLag == 0 {
		return ""
	}

	return targetField
}

func (p *processor) startOldProcessedReferralsCleaner(ctx context.Context) {
	ticker := stdlibtime.NewTicker(stdlibtime.Duration(1+rand.Intn(24)) * stdlibtime.Minute) //nolint:gosec,gomnd // Not an  issue.
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			const deadline = 30 * stdlibtime.Second
			reqCtx, cancel := context.WithTimeout(ctx, deadline)
			log.Error(errors.Wrap(p.deleteOldProcessedReferrals(reqCtx), "failed to deleteOldTrackedActions"))
			cancel()
		case <-ctx.Done():
			return
		}
	}
}

func (p *processor) deleteOldProcessedReferrals(ctx context.Context) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline")
	}
	sql := `DELETE FROM processed_referrals WHERE processed_at < $1`
	if _, err := storage.Exec(ctx, p.db, sql, time.Now().Add(-24*stdlibtime.Hour)); err != nil {
		return errors.Wrap(err, "failed to delete old data from processed_referrals")
	}

	return nil
}
