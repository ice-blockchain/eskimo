// SPDX-License-Identifier: ice License 1.0

package emaillinkiceauth

import (
	"context"
	"strings"

	"dario.cat/mergo"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/terror"
	"github.com/ice-blockchain/wintr/time"
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
	randomID := iceIDPrefix + uuid.NewString()
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
	sql := `SELECT id FROM (
				SELECT users.id, 1 as idx
					FROM users 
						WHERE email = $1
				UNION ALL
				(SELECT COALESCE(user_id, $2) AS id, 2 as idx
					FROM email_link_sign_ins
						WHERE email = $1)
			) t ORDER BY idx LIMIT 1`
	ids, err := storage.Select[dbUserID](ctx, c.db, sql, searchEmail, idIfNotFound)
	if err != nil || len(ids) == 0 {
		if storage.IsErr(err, storage.ErrNotFound) || (err == nil && len(ids) == 0) {
			return idIfNotFound, nil
		}

		return "", errors.Wrapf(err, "failed to find user by email:%v", searchEmail)
	}

	return ids[0].ID, nil
}

func (c *client) isUserExist(ctx context.Context, email string) error {
	type dbUser struct {
		ID string
	}
	sql := `SELECT id 
				FROM users 
					WHERE email = $1`
	_, err := storage.Get[dbUser](ctx, c.db, sql, email)

	return errors.Wrapf(err, "failed to find user by email:%v", email)
}

//nolint:funlen // .
func (c *client) getUserByIDOrPk(ctx context.Context, userID string, id *loginID) (*emailLinkSignIn, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user by id or email failed because context failed")
	}
	usr, err := storage.Get[emailLinkSignIn](ctx, c.db, `
		SELECT 		created_at,
					token_issued_at,
					COALESCE(previously_issued_token_seq, 0) 			AS previously_issued_token_seq, 
					COALESCE(issued_token_seq, 0) 			 			AS issued_token_seq,
					blocked_until,
					COALESCE(confirmation_code_wrong_attempts_count, 0) AS confirmation_code_wrong_attempts_count,
					otp,
					confirmation_code,
					user_id,
					email,
					device_unique_id,
					language,
		    		hash_code,
		    		metadata
		FROM (
			WITH emails AS (
				SELECT
					created_at,
					token_issued_at,
		    		previously_issued_token_seq,
					issued_token_seq,
					blocked_until,
					confirmation_code_wrong_attempts_count,
					otp,
					confirmation_code,
					$1 												   AS user_id,
					email,
					$3 												   AS device_unique_id,
					'en' 											   AS language,
					COALESCE((account_metadata.metadata -> 'hash_code')::BIGINT,0) AS hash_code,
					account_metadata.metadata,
					2                                                  AS idx
				FROM email_link_sign_ins
				LEFT JOIN account_metadata ON account_metadata.user_id = $1
				WHERE email = $2 AND device_unique_id = $3
			)
			SELECT
					emails.created_at                                  AS created_at,
					emails.token_issued_at       			 	  	   AS token_issued_at,
		    		emails.previously_issued_token_seq                 AS previously_issued_token_seq,
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
					account_metadata.metadata    				 	   AS metadata,
					1 												   AS idx
				FROM users u
				LEFT JOIN emails ON emails.email = $2 and u.id = emails.user_id
				LEFT JOIN account_metadata ON u.id = account_metadata.user_id
				WHERE u.id = $1
			UNION ALL (select * from emails)
		) t ORDER BY idx LIMIT 1
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

//nolint:revive // .
func (c *client) getEmailLinkSignIn(ctx context.Context, id *loginID, fromMaster bool) (*emailLinkSignIn, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get user by id or email failed because context failed")
	}
	sql := `SELECT *
			FROM email_link_sign_ins
			WHERE email = $1
				  AND device_unique_id = $2`
	var (
		signIn *emailLinkSignIn
		err    error
	)
	if fromMaster {
		signIn, err = storage.ExecOne[emailLinkSignIn](ctx, c.db, sql, id.Email, id.DeviceUniqueID)
	} else {
		signIn, err = storage.Get[emailLinkSignIn](ctx, c.db, sql, id.Email, id.DeviceUniqueID)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get email sign in by id:%#v", id)
	}

	return signIn, nil
}

func (c *client) IceUserID(ctx context.Context, email string) (string, error) {
	if email == "" {
		return "", nil
	}
	userID, err := c.getUserIDFromEmail(ctx, email, "")
	if err != nil {
		return "", errors.Wrapf(err, "failed to fetch userID by email:%v", email)
	}
	if strings.HasPrefix(userID, iceIDPrefix) {
		return userID, nil
	}

	return "", nil
}

//nolint:funlen // .
func (c *client) UpdateMetadata(ctx context.Context, userID string, newData *users.JSON) (*users.JSON, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "update account metadata failed because context failed")
	}
	md, err := storage.Get[metadata](ctx, c.db, `SELECT * FROM account_metadata WHERE user_id = $1`, userID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, errors.Wrapf(err, "failed to get account metadata for userID %v", userID)
	}
	if md == nil {
		md = &metadata{Metadata: nil}
	}
	prev := md.Metadata
	if md.Metadata == nil {
		empty := users.JSON(map[string]any{})
		md.Metadata = &empty
	}
	if err = mergo.Merge(newData, md.Metadata, mergo.WithOverride, mergo.WithTypeCheck); err != nil {
		return nil, errors.Wrapf(err, "failed to merge %#v and %#v", md, newData)
	}
	sql := `INSERT INTO account_metadata(user_id, metadata)
 				VALUES ($1, $2) 
				ON CONFLICT(user_id) DO UPDATE
					SET metadata = EXCLUDED.metadata
			WHERE account_metadata.metadata = $3::jsonb`
	var rowsUpdated uint64
	rowsUpdated, err = storage.Exec(ctx, c.db, sql, userID, newData, prev)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update account metadata for userID:%v to %#v", userID, newData)
	}
	if rowsUpdated == 0 {
		return c.UpdateMetadata(ctx, userID, newData)
	}

	return newData, nil
}

func (c *client) Metadata(ctx context.Context, userID, tokenEmail string) (string, *users.JSON, error) {
	md, err := storage.Get[metadata](ctx, c.db, `
	SELECT COALESCE(user_id, id) as user_id, metadata, email FROM (
		SELECT account_metadata.*, u.email, u.id, 1 as idx 
		  FROM users u 
		  LEFT JOIN account_metadata ON account_metadata.user_id = $1
		  WHERE u.id = $1
		UNION ALL (
		  SELECT account_metadata.*, u.email, u.id, 2 as idx
		  FROM account_metadata 
		  LEFT JOIN users u ON u.id = $1
		  WHERE account_metadata.user_id = $1
		)
    ) t ORDER BY idx LIMIT 1`, userID)
	if err != nil {
		return "", nil, errors.Wrapf(err, "failed to get user metadata %v", userID)
	}
	if md.Email != nil {
		emailEmpty := *md.Email == "" || *md.Email == *md.UserID
		if tokenEmail != "" && !emailEmpty && !strings.EqualFold(tokenEmail, *md.Email) { //nolint:gosec // .
			return "", nil, terror.New(ErrUserDataMismatch, map[string]any{"email": *md.Email})
		}
	}
	encoded := ""
	if md.Metadata != nil {
		if encoded, err = c.authClient.GenerateMetadata(time.Now(), userID, *md.Metadata); err != nil {
			return "", nil, errors.Wrapf(err, "failed to encode metadata:%#v for userID:%v", md, userID)
		}
	}

	return encoded, md.Metadata, nil
}
