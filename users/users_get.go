// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"

	"github.com/cenkalti/backoff/v4"
	"github.com/framey-io/go-tarantool"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/ice-blockchain/wintr/time"
)

func (r *repository) mustGetUserByID(ctx context.Context, userID UserID) (usr *User, err error) {
	if usr, err = r.getUserByID(ctx, userID); err == nil {
		return
	}

	if !errors.Is(err, ErrNotFound) {
		return nil, errors.Wrapf(err, "failed to get current user for userID:%v", userID)
	}

	err = retry(ctx, func() error {
		if usr, err = r.getUserByID(ctx, userID); err != nil {
			if errors.Is(err, ErrNotFound) {
				return err
			}

			//nolint:wrapcheck // It's a proxy.
			return backoff.Permanent(err)
		}

		return nil
	})

	return
}

func (r *repository) getUserByID(ctx context.Context, id UserID) (*User, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user failed because context failed")
	}
	result := new(User)
	if err := r.db.GetTyped("USERS", "pk_unnamed_USERS_1", tarantool.StringKey{S: id}, result); err != nil {
		return nil, errors.Wrapf(err, "failed to get user by id %v", id)
	}
	if result.ID == "" {
		return nil, ErrNotFound
	}
	result.setCorrectProfilePictureURL()

	return result, nil
}

func (r *repository) GetUserByID(ctx context.Context, userID string) (*UserProfile, error) { //nolint:revive // Its fine.
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user failed because context failed")
	}
	if userID != requestingUserID(ctx) {
		return r.getOtherUserProfile(ctx, userID)
	}
	sql := `
	SELECT  u.*,
			count(distinct t1.id) + count(t2.id) AS total_referral_count
	FROM users u 
			LEFT JOIN USERS t1
                	ON t1.referred_by = u.id
					and t1.id != u.id
						LEFT JOIN USERS t2
								ON t2.referred_by = t1.id
								and t2.id != t1.id
	WHERE u.id = :userId`
	var result []*UserProfile
	if err := r.db.PrepareExecuteTyped(sql, map[string]interface{}{"userId": userID}, &result); err != nil {
		return nil, errors.Wrapf(err, "failed to select user by id %v", userID)
	}
	if len(result) == 0 || result[0].ID == "" { //nolint:revive // False negative.
		return nil, errors.Wrapf(ErrNotFound, "no user found with id %v", userID)
	}
	result[0].setCorrectProfilePictureURL()

	return result[0], nil
}

func (r *repository) getOtherUserProfile(ctx context.Context, userID string) (*UserProfile, error) {
	usr, err := r.getUserByID(ctx, userID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to select user by id %v", userID)
	}
	usr.PhoneNumber = ""

	return &UserProfile{User: User{PublicUserInformation: usr.PublicUserInformation}}, nil
}

func (r *repository) GetUserByUsername(ctx context.Context, username string) (*UserProfile, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user failed because context failed")
	}
	result := new(User)
	if err := r.db.GetTyped("USERS", "unique_unnamed_USERS_2", tarantool.StringKey{S: username}, result); err != nil {
		return nil, errors.Wrapf(err, "failed to get user by username %v", username)
	}
	if result.ID == "" {
		return nil, errors.Wrapf(ErrNotFound, "no user found with username %v", username)
	}
	result.setCorrectProfilePictureURL()
	if result.ID != requestingUserID(ctx) {
		result.PhoneNumber = ""

		return &UserProfile{User: User{PublicUserInformation: result.PublicUserInformation}}, nil
	}

	return &UserProfile{User: *result}, nil
}

//nolint:funlen // Big sql.
func (r *repository) GetUsers(ctx context.Context, keyword string, limit, offset uint64) (result []*RelatableUserProfile, err error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get users failed because context failed")
	}
	sql := fmt.Sprintf(`
			SELECT u.last_mining_started_at                                                                          AS last_mining_started_at,
				   (CASE
						WHEN t0.id = :userId
							THEN u.last_ping_at
						ELSE :nowNanos
					END)                                                                                             AS last_ping_at,	
				   u.id                                                                                              AS id,
				   u.username                                                                                        AS username,
				   u.first_name                                                                                      AS first_name,
				   u.last_name                                                                                       AS last_name,
				   (SELECT u.phone_number
					FROM users user_requesting_this
					WHERE 1=1
						AND user_requesting_this.id = :userId
						AND (POSITION(u.phone_number_hash, user_requesting_this.agenda_phone_number_hashes) > 0
								OR
							 user_requesting_this.id == u.id))      												 AS phone_number_,
				   '%v/' || u.profile_picture_name                                                                   AS profile_picture_url,
				   u.country                                                                                         AS country,
				   u.city                                                                                            AS city,
				   (CASE
						WHEN t0.id = :userId and t0.id != u.id
							THEN 'T1'
						WHEN t0.referred_by = :userId and t0.id != t0.referred_by
							THEN 'T2'
						ELSE ''
					END)                                                                                             AS referral_type	
			FROM users u
					 JOIN USERS t0
						  ON t0.id = u.referred_by
			WHERE (
					POSITION(LOWER(:keyword),LOWER(u.username)) = 1
					OR
					POSITION(LOWER(:keyword),LOWER(u.first_name)) = 1
					OR
					POSITION(LOWER(:keyword),LOWER(u.last_name)) = 1
				  )
			ORDER BY
				(phone_number_ != '' AND phone_number_ != null) DESC,
				t0.id = :userId DESC,
				t0.referred_by = :userId DESC,
				u.username DESC
			LIMIT %v OFFSET :offset`, cfg.PictureStorage.URLDownload, limit)
	params := map[string]interface{}{
		"keyword":  keyword,
		"offset":   offset,
		"userId":   requestingUserID(ctx),
		"nowNanos": time.Now(),
	}
	err = r.db.PrepareExecuteTyped(sql, params, &result)

	return result, errors.Wrapf(err, "failed to select for users by %#v", params)
}

func (n *NotExpired) DecodeMsgpack(dec *msgpack.Decoder) error {
	v := new(time.Time)
	if err := v.DecodeMsgpack(dec); err != nil {
		return errors.Wrap(err, "failed to decode time based struct for NotExpired")
	}
	*n = time.Now().Sub(*v.Time) <= expirationDeadline

	return nil
}

func (i *PublicUserInformation) setCorrectProfilePictureURL() {
	i.ProfilePictureURL = fmt.Sprintf("%v/%v", cfg.PictureStorage.URLDownload, i.ProfilePictureURL)
}
