// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage/v2"
)

func (c *client) getUserByPk(ctx context.Context, id *ID, oldEmail string) (*minimalUser, error) {
	userID, err := c.findUser(ctx, id.Email, oldEmail)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch or generate userID for email:%v", id.Email)
	}
	usr, err := c.getUserByIDOrPk(ctx, userID, id)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user info by userID:%v", userID)
	}

	return usr, nil
}

func (c *client) findUser(ctx context.Context, email, oldEmail string) (userID string, err error) {
	if ctx.Err() != nil {
		return "", errors.Wrap(ctx.Err(), "find or generate user by id or email context failed")
	}
	randomID := uuid.NewString()
	searchEmail := email
	if oldEmail != "" {
		searchEmail = oldEmail
	}
	type dbUserID struct {
		ID string
	}
	sql := `SELECT 
				id 
			FROM users 
			WHERE email = $1 OR id = $2`
	ids, err := storage.Select[dbUserID](ctx, c.db, sql, searchEmail, randomID)
	if err != nil || len(ids) == 0 {
		if storage.IsErr(err, storage.ErrNotFound) || len(ids) == 0 {
			return randomID, nil
		}

		return "", errors.Wrapf(err, "failed to find user by userID:%v or email:%v", randomID, email)
	}
	if ids[0].ID == randomID || (len(ids) > 1) {
		return c.findUser(ctx, email, oldEmail)
	}

	return ids[0].ID, nil
}

//nolint:funlen // .
func (c *client) getUserByIDOrPk(ctx context.Context, userID string, id *ID) (*minimalUser, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user by id or email failed because context failed")
	}
	usr, err := storage.Get[minimalUser](ctx, c.db, `
		WITH emails AS (
			SELECT
				confirmation_code_created_at,
				token_issued_at,
				issued_token_seq,
				confirmation_code_wrong_attempts_count,
				otp,
				login_session,
				confirmation_code,
				$1 												   AS user_id,
				email,
				$3 												   AS device_unique_id,
				'en' 											   AS language,
				COALESCE((custom_claims -> 'hash_code')::BIGINT,0) AS hash_code,
				custom_claims
			FROM email_link_sign_ins
			WHERE email = $2 AND device_unique_id = $3
		)
		SELECT 
				emails.confirmation_code_created_at      	  	   AS confirmation_code_created_at,
				emails.token_issued_at       			 	  	   AS token_issued_at,
				emails.issued_token_seq       			 	  	   AS issued_token_seq,
				emails.confirmation_code_wrong_attempts_count 	   AS confirmation_code_wrong_attempts_count,
				emails.otp       						 	  	   AS otp,
				emails.login_session       		 	  		  	   AS login_session,
				emails.confirmation_code       			 	  	   AS confirmation_code,
				u.id 									 	  	   AS user_id,
				u.email,
				emails.device_unique_id 				 	  	   AS device_unique_id,
				u.language			    				 	  	   AS language,
				u.hash_code,
				emails.custom_claims    				 	  	   AS custom_claims
			FROM users u, emails
			WHERE u.id = $1
		UNION ALL (select * from emails)
		LIMIT 1
	`, userID, id.Email, id.DeviceUniqueID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user by id:%v or id:%#v)", id, id)
	}

	return usr, nil
}

func (c *client) getUserByLoginSession(ctx context.Context, loginSession string, id *ID) (*minimalUser, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user by id or email failed because context failed")
	}
	sql := `SELECT *
			FROM email_link_sign_ins
			WHERE login_session = $1 
	  			  AND email = $2
				  AND device_unique_id = $3`
	usr, err := storage.Get[minimalUser](ctx, c.db, sql, loginSession, id.Email, id.DeviceUniqueID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user by loginSession:%v and id:%#v", loginSession, id)
	}

	return usr, nil
}
