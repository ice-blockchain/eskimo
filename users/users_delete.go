// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage/v2"
)

func (r *repository) DeleteUser(ctx context.Context, userID UserID) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	gUser, err := r.getUserByID(ctx, userID)
	if err != nil {
		return errors.Wrapf(err, "failed to get user for userID:%v", userID)
	}
	if err = r.deleteUser(ctx, gUser); err != nil {
		return errors.Wrapf(err, "failed to deleteUser for:%#v", gUser)
	}
	u := &UserSnapshot{Before: r.sanitizeUser(gUser)}
	if err = r.sendUserSnapshotMessage(ctx, u); err != nil {
		return errors.Wrapf(err, "failed to send deleted user message for %#v", u)
	}
	if err = r.sendTombstonedUserMessage(ctx, userID); err != nil {
		return errors.Wrapf(err, "failed to sendTombstonedUserMessage for userID:%v", userID)
	}

	return nil
}

func (r *repository) deleteUser(ctx context.Context, usr *User) error { //nolint:revive // .
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "delete user failed because context failed")
	}
	if err := r.deleteUserReferences(ctx, usr.ID); err != nil {
		return errors.Wrapf(err, "failed to deleteUserReferences for userID:%v", usr.ID)
	}
	if err := r.updateReferredByForAllT1Referrals(ctx, usr.ID); err != nil {
		for err != nil && (errors.Is(err, storage.ErrRelationNotFound) || errors.Is(err, storage.ErrNotFound)) {
			err = r.updateReferredByForAllT1Referrals(ctx, usr.ID)
		}
		if err != nil {
			return errors.Wrapf(err, "failed to update referredBy for all t1 referrals of userID:%v", usr.ID)
		}
	}
	gUser, err := r.getUserByID(ctx, usr.ID)
	if err != nil {
		return errors.Wrapf(err, "failed to get user for userID:%v", usr.ID)
	}
	*usr = *gUser
	sql := `DELETE FROM users WHERE id = $1`
	if _, tErr := storage.Exec(ctx, r.db, sql, usr.ID); tErr != nil {
		if errors.Is(tErr, storage.ErrRelationNotFound) {
			return r.deleteUser(ctx, usr)
		}

		return errors.Wrapf(tErr, "failed to delete user with id %v", usr.ID)
	}

	return nil
}

func (r *repository) deleteUserReferences(ctx context.Context, userID UserID) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "delete user failed because context failed")
	}
	wg := new(sync.WaitGroup)
	wg.Add(1)
	errChan := make(chan error, 1)
	go func() {
		defer wg.Done()
		errChan <- errors.Wrapf(r.DeleteAllDeviceMetadata(ctx, userID), "failed to DeleteAllDeviceMetadata for userID:%v", userID)
	}()
	wg.Wait()
	close(errChan)
	errs := make([]error, 0, 1)
	for err := range errChan {
		errs = append(errs, err)
	}

	return multierror.Append(nil, errs...).ErrorOrNil() //nolint:wrapcheck // Not needed.
}

//nolint:funlen // It's better to isolate everything together to decrease complexity; and it has some SQL, so...
func (r *repository) updateReferredByForAllT1Referrals(ctx context.Context, userID UserID) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	sql := `SELECT (	SELECT X.ID 
						FROM (	SELECT X.ID 
								FROM (  SELECT r.id 
										FROM users r
										WHERE 1=1
											  AND r.id != $1
											  AND r.id != u.id 
											  AND r.referred_by != u.id 
											  AND r.referred_by != r.id 
											  AND r.username != r.id 
											  AND r.referred_by != $1
										ORDER BY RANDOM() 
										LIMIT 1
									 ) X
			
								UNION ALL 
								 
								SELECT u.id AS ID
							  ) X
						LIMIT 1
				   ) new_referred_by,
				   u.created_at,
				   u.updated_at,
				   u.last_mining_started_at,
				   u.last_mining_ended_at,
				   u.last_ping_cooldown_ended_at,
				   COALESCE(u.hidden_profile_elements, '') 	  AS hidden_profile_elements,
				   u.random_referred_by,
				   u.verified,
				   COALESCE(u.client_data, '') 				  AS client_data,
				   COALESCE(u.phone_number, '') 			  AS phone_number,
				   COALESCE(u.email, '') 					  AS email,
				   COALESCE(u.first_name, '') 				  AS first_name,
				   COALESCE(u.last_name, '') 				  AS last_name,
				   u.country,
				   u.city,
				   u.id,
				   COALESCE(u.username, '') 				  AS username,
				   u.profile_picture_name 					  AS profile_picture_url,
				   u.referred_by,
				   COALESCE(u.phone_number_hash, '') 		  AS phone_number_hash,
				   COALESCE(u.agenda_phone_number_hashes, '') AS agenda_phone_number_hashes,
				   u.mining_blockchain_account_address,
				   u.blockchain_account_address,
				   u.language,
				   u.hash_code
			FROM users u
			WHERE u.referred_by = $1
			  AND u.id != $1`
	type resp struct {
		NewReferredBy UserID
		User
	}
	res, err := storage.Select[resp](ctx, r.db, sql, userID)
	if err != nil {
		return errors.Wrapf(err, "failed to select for all t1 referrals of userID:%v + their new random referralID", userID)
	}

	wg := new(sync.WaitGroup)
	wg.Add(len(res))
	errChan := make(chan error, len(res))
	for ii := range res {
		go func(ix int) {
			defer wg.Done()
			errChan <- errors.Wrapf(r.updateReferredBy(ctx, &res[ix].User, res[ix].NewReferredBy, true),
				"failed to update referred by for userID:%v", res[ix].User.ID)
		}(ii)
	}
	wg.Wait()
	close(errChan)
	errs := make([]error, 0, len(res))
	for err := range errChan {
		errs = append(errs, err)
	}

	return errors.Wrap(multierror.Append(nil, errs...).ErrorOrNil(), "failed to update referred by for some/all of user's t1 referrals")
}

func (r *repository) deleteUserTracking(ctx context.Context, usr *UserSnapshot) error {
	if usr.Before != nil && usr.User == nil {
		return errors.Wrapf(r.trackingClient.DeleteUser(ctx, usr.Before.ID), "failed to delete tracking data for userID:%v", usr.Before.ID)
	}

	return nil
}
