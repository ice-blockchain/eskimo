// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/time"
)

func (r *repository) getUserByID(ctx context.Context, id UserID) (*User, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user failed because context failed")
	}
	result, err := storage.Get[User](ctx, r.db, `
	SELECT users.*,
	       qs.user_id IS NOT NULL AND qs.ended_at is not null AND qs.ended_successfully = true AS quiz_completed
		FROM users
		LEFT JOIN quiz_sessions qs
			ON qs.user_id = users.id
		WHERE id = $1`, id)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user by id %v", id)
	}

	return result, nil
}

func (r *repository) GetUserByID(ctx context.Context, userID string) (*UserProfile, error) { //nolint:revive // Its fine.
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user failed because context failed")
	}
	if userID != requestingUserID(ctx) {
		return r.getOtherUserByID(ctx, userID)
	}
	sql := `
		SELECT  	
			u.*,
			(qs.user_id IS NOT NULL AND qs.ended_at is not null AND qs.ended_successfully = true) AS quiz_completed,
			COALESCE(refs.t1, 0) 		  as t1_referral_count,
			COALESCE(refs.t2, 0)		  as t2_referral_count
		FROM users u 
				LEFT JOIN referral_acquisition_history refs
						ON refs.user_id = u.id
				LEFT JOIN quiz_sessions qs
					ON qs.user_id = u.id
		WHERE u.id = $1`
	res, err := storage.Get[UserProfile](ctx, r.db, sql, userID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to select user by id %v", userID)
	}
	r.sanitizeUser(res.User)
	r.sanitizeUserForUI(res.User)

	return res, nil
}

func (r *repository) getOtherUserByID(ctx context.Context, userID string) (*UserProfile, error) { //nolint:funlen // Better to be in one place.
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user failed because context failed")
	}
	usr, err := r.getUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	verified := usr.IsVerified()
	*usr = User{
		HiddenProfileElements: usr.HiddenProfileElements,
		PublicUserInformation: usr.PublicUserInformation,
		Verified:              &verified,
	}
	referralCountNeeded := true
	if usr.HiddenProfileElements != nil {
		for _, element := range *usr.HiddenProfileElements {
			if element == ReferralCountHiddenProfileElement {
				referralCountNeeded = false

				break
			}
		}
	}
	if !referralCountNeeded {
		resp := new(UserProfile)
		resp.User = r.sanitizeUser(usr)

		return resp, nil
	}

	sql := `SELECT  u.id,
					COALESCE(refs.t1, 0) AS t1_referral_count,
					COALESCE(refs.t2, 0) 		  AS t2_referral_count
			FROM users u 
				LEFT JOIN referral_acquisition_history refs
						ON refs.user_id = u.id
			WHERE u.id = $1`
	type result struct {
		ID              string
		T1ReferralCount uint64
		T2ReferralCount uint64
	}
	dbRes, err := storage.Get[result](ctx, r.db, sql, userID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to select referralCount for user by id %v", userID)
	}
	resp := new(UserProfile)
	resp.T1ReferralCount = &dbRes.T1ReferralCount
	resp.T2ReferralCount = &dbRes.T2ReferralCount
	resp.User = r.sanitizeUser(usr)

	return resp, nil
}

func (r *repository) GetUserByUsername(ctx context.Context, username string) (*UserProfile, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user failed because context failed")
	}
	result, err := storage.Get[User](ctx, r.db, `
		SELECT users.*, 
       		   (qs.user_id IS NOT NULL AND qs.ended_at is not null AND qs.ended_successfully = true) AS quiz_completed
		FROM users 
		LEFT JOIN quiz_sessions qs
			ON qs.user_id = users.id
		WHERE username = $1`, username)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user by username %v", username)
	}
	resp := new(UserProfile)
	resp.User = new(User)
	resp.PublicUserInformation = result.PublicUserInformation
	r.sanitizeUser(resp.User)
	r.sanitizeUserForUI(resp.User)

	return resp, nil
}

func (r *repository) GetUserByPhoneNumber(ctx context.Context, phoneNumber string) (*User, error) {
	sql := `SELECT users.*, (qs.user_id IS NOT NULL AND qs.ended_at is not null AND qs.ended_successfully = true) AS quiz_completed
			FROM users 
			LEFT JOIN quiz_sessions qs
					ON qs.user_id = users.id
			WHERE phone_number = $1 AND phone_number != id`
	usr, err := storage.Get[User](ctx, r.db, sql, phoneNumber)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil //nolint:nilnil // Nope.
		}

		return nil, errors.Wrapf(err, "failed to get user by phoneNumber `%v`", phoneNumber)
	}
	r.sanitizeUser(usr)
	r.sanitizeUserForUI(usr)

	return usr, nil
}

func (r *repository) IsEmailUsedBySomebodyElse(ctx context.Context, userID, email string) (bool, error) {
	sql := `SELECT id FROM users where email = $1`
	usr, err := storage.Get[struct{ ID string }](ctx, r.db, sql, email)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return false, nil
		}

		return false, errors.Wrapf(err, "failed to check email ownership for userID:%v,email:%v", userID, email)
	}
	if usr.ID == userID {
		return false, ErrDuplicate
	}

	return true, nil
}

//nolint:funlen // Big sql.
func (r *repository) GetUsers(ctx context.Context, keyword string, limit, offset uint64) (result []*MinimalUserProfile, err error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get users failed because context failed")
	}
	sql := fmt.Sprintf(`
			SELECT 
			    (u.kyc_step_passed >= %[2]v AND u.quiz_completed) 				  AS verified,
			    u.last_mining_ended_at 									 	 	  AS active,
			    u.last_ping_cooldown_ended_at 							 	 	  AS pinged,
			    u.phone_number_ 										  		  AS phone_number,
			    u.email 												  		  AS email,
			    u.id 												 	  		  AS id,
				u.username 												  		  AS username,
				u.profile_picture_url 									  		  AS profile_picture_name,
				u.country 											  	  		  AS country,
				u.city 													  		  AS city,
			    u.referral_type 										  		  AS referral_type
			FROM (SELECT COALESCE(u.last_mining_ended_at,to_timestamp(1)) 		  AS last_mining_ended_at,
				   (CASE
						WHEN user_requesting_this.id != u.id AND (u.referred_by = user_requesting_this.id OR u.id = user_requesting_this.referred_by)
							THEN (CASE 
									WHEN COALESCE(u.last_mining_ended_at,to_timestamp(0)) < $1 
									    THEN COALESCE(u.last_ping_cooldown_ended_at,to_timestamp(1)) 
								   	ELSE u.last_mining_ended_at 
							      END)
						WHEN t0.referred_by = user_requesting_this.id and t0.id != t0.referred_by
							THEN
								null
						ELSE to_timestamp(0)
					END) 		AS last_ping_cooldown_ended_at,
				   (CASE
						WHEN user_requesting_this.id = u.id 
								OR (
									NULLIF(u.phone_number_hash,'') IS NOT NULL
									AND 
									u.id = ANY(user_requesting_this.agenda_contact_user_ids)
								   )
							THEN u.phone_number
						ELSE ''
				    END) 														  AS phone_number_,
				   ''           												  AS email,
				   u.id         												  AS id,
				   u.username   												  AS username,
				   %[1]v           												  AS profile_picture_url,
				   u.country 													  AS country,
				   '' 															  AS city,
			       u.referred_by 												  AS referred_by,
			       u.kyc_step_passed 											  AS kyc_step_passed,
				   (CASE
						WHEN NULLIF(u.phone_number_hash,'') IS NOT NULL
				  				AND user_requesting_this.id != u.id
								AND u.id = ANY(user_requesting_this.agenda_contact_user_ids)
							THEN 'CONTACTS'
						WHEN u.id = user_requesting_this.referred_by OR u.referred_by = user_requesting_this.id 
							THEN 'T1'
						WHEN t0.referred_by = user_requesting_this.id and t0.id != t0.referred_by
							THEN 'T2'
						ELSE ''
					END) 														                        AS referral_type,
			        user_requesting_this.id                                                             AS user_requesting_this_id,
				    user_requesting_this.referred_by                                                    AS user_requesting_this_referred_by,
				    t0.referred_by                                                                      AS t0_referred_by,
				    t0.id                                                                               AS t0_id,
			        qs.user_id IS NOT NULL AND qs.ended_at is not null AND qs.ended_successfully = true AS quiz_completed
			FROM users u
					 JOIN USERS t0
						  ON t0.id = u.referred_by
						 AND t0.referred_by != t0.id
						 AND t0.username != t0.id
					 JOIN users user_requesting_this
						  ON user_requesting_this.id = $5
						 AND user_requesting_this.username != user_requesting_this.id
						 AND user_requesting_this.referred_by != user_requesting_this.id
				     LEFT JOIN quiz_sessions qs
					   ON qs.user_id = u.id
			WHERE 
					u.lookup @@ $2::tsquery
				  ) u 
				  WHERE referral_type != '' AND u.username != u.id AND u.referred_by != u.id
				  ORDER BY
							u.id = u.user_requesting_this_referred_by DESC,
							(phone_number_ != '' AND phone_number_ is not null) DESC,
							u.t0_id = u.user_requesting_this_id DESC,
							u.t0_referred_by = u.user_requesting_this_id DESC,
							u.username DESC
			LIMIT $3 OFFSET $4`, r.pictureClient.SQLAliasDownloadURL(`u.profile_picture_name`), QuizKYCStep)
	params := []any{
		time.Now().Time,
		strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(keyword), "_", "\\_"), "%", "\\%"),
		limit,
		offset,
		requestingUserID(ctx),
	}
	result, err = storage.Select[MinimalUserProfile](ctx, r.db, sql, params...)
	if result == nil {
		result = []*MinimalUserProfile{}
	}

	return result, errors.Wrapf(err, "failed to select for users by %#v", params...)
}
