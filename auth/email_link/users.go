// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage/v2"
)

func (c *client) getEmailLinkSignInByPk(ctx context.Context, id *loginID, oldEmail string) (*emailLinkSignIn, error) {
	userID, err := c.findOrGenerateUserID(ctx, id.Email, oldEmail)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch or generate userID for email:%v", id.Email)
	}
	usr, err := c.getUserByIDOrPk(ctx, userID, id)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user info by userID:%v", userID)
	}

	return usr, nil
}

func (c *client) findOrGenerateUserID(ctx context.Context, email, oldEmail string) (userID string, err error) {
	if ctx.Err() != nil {
		return "", errors.Wrap(ctx.Err(), "find or generate user by id or email context failed")
	}
	randomID := uuid.NewString()
	searchEmail := email
	if oldEmail != "" {
		searchEmail = oldEmail
	}

	return c.getUserIDFromEmail(ctx, searchEmail, randomID)
}

func (c *client) getUserIDFromEmail(ctx context.Context, searchEmail, idIfNotFound string) (userID string, err error) {
	type dbUserID struct {
		ID string
	}
	sql := `SELECT id 
				FROM users 
					WHERE email = $1
			UNION ALL
			SELECT COALESCE(user_id, $2) AS id 
				FROM email_link_sign_ins
					WHERE email = $1
			LIMIT 1`
	ids, err := storage.Select[dbUserID](ctx, c.db, sql, searchEmail, idIfNotFound)
	if err != nil || len(ids) == 0 {
		if storage.IsErr(err, storage.ErrNotFound) || len(ids) == 0 {
			return idIfNotFound, nil
		}

		return "", errors.Wrapf(err, "failed to find user by email:%v", searchEmail)
	}

	return ids[0].ID, nil
}

//nolint:funlen // .
func (c *client) getUserByIDOrPk(ctx context.Context, userID string, id *loginID) (*emailLinkSignIn, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user by id or email failed because context failed")
	}
	usr, err := storage.Get[emailLinkSignIn](ctx, c.db, `
		WITH emails AS (
			SELECT
				token_issued_at,
				issued_token_seq,
				blocked_until,
				confirmation_code_wrong_attempts_count,
				otp,
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
				emails.token_issued_at       			 	  	   AS token_issued_at,
				emails.issued_token_seq       			 	  	   AS issued_token_seq,
				emails.blocked_until       			 	  	   	   AS blocked_until,
				emails.confirmation_code_wrong_attempts_count 	   AS confirmation_code_wrong_attempts_count,
				emails.otp       						 	  	   AS otp,
				emails.confirmation_code       			 	  	   AS confirmation_code,
				u.id 									 	  	   AS user_id,
				u.email,
				emails.device_unique_id 				 	  	   AS device_unique_id,
				u.language			    				 	  	   AS language,
				u.hash_code,
				emails.custom_claims    				 	  	   AS custom_claims
			FROM users u
			JOIN emails ON u.email = emails.email and u.id = emails.user_id
			WHERE u.id = $1
		UNION ALL (select * from emails)
		LIMIT 1
	`, userID, id.Email, id.DeviceUniqueID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user by pk:%#v)", id)
	}

	return usr, nil
}

func (c *client) getConfirmedEmailLinkSignIn(ctx context.Context, id *loginID, confirmationCode string) (*emailLinkSignIn, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user by id or email failed because context failed")
	}
	sql := `SELECT *
			FROM email_link_sign_ins
			WHERE confirmation_code = $1 
	  			  AND email = $2
				  AND device_unique_id = $3`
	usr, err := storage.Get[emailLinkSignIn](ctx, c.db, sql, confirmationCode, id.Email, id.DeviceUniqueID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user by confirmation code:%v and id:%#v", confirmationCode, id)
	}

	return usr, nil
}

func (c *client) getEmailLinkSignIn(ctx context.Context, id *loginID) (*emailLinkSignIn, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user by id or email failed because context failed")
	}
	sql := `SELECT *
			FROM email_link_sign_ins
			WHERE email = $1
				  AND device_unique_id = $2`
	usr, err := storage.Get[emailLinkSignIn](ctx, c.db, sql, id.Email, id.DeviceUniqueID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get email sign in by id:%#v", id)
	}

	return usr, nil
}

func (c *client) IceUserID(ctx context.Context, email string) (string, error) {
	userID, err := c.getUserIDFromEmail(ctx, email, "")
	if err != nil {
		return "", errors.Wrapf(err, "failed to fetch userID by email:%v", email)
	}

	return userID, nil
}
