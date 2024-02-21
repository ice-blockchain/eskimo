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
		for err != nil && (storage.IsErr(err, storage.ErrRelationNotFound) || storage.IsErr(err, storage.ErrNotFound)) {
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
		if storage.IsErr(tErr, storage.ErrRelationNotFound) || storage.IsErr(tErr, storage.ErrRelationInUse) {
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
	errChan := make(chan error, 2) //nolint:gomnd // .
	go func() {
		defer wg.Done()
		errChan <- errors.Wrapf(r.DeleteAllDeviceMetadata(ctx, userID), "failed to DeleteAllDeviceMetadata for userID:%v", userID)
		errChan <- errors.Wrapf(r.deleteReferralAcquisitionHistory(ctx, userID), "failed to deleteReferralAcquisitionHistory for userID:%v", userID)
	}()
	wg.Wait()
	close(errChan)
	errs := make([]error, 0, len(errChan))
	for err := range errChan {
		errs = append(errs, err)
	}

	return multierror.Append(nil, errs...).ErrorOrNil() //nolint:wrapcheck // Not needed.
}

func (r *repository) updateReferredByForAllT1Referrals(ctx context.Context, userID UserID) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	sql := `
		UPDATE users SET
		    referred_by = $2,
		    random_referred_by = true
		WHERE referred_by = $1
			AND id != $1
		    AND id != 'bogus'
			AND id != 'icenetwork' 
		    AND referred_by != id`
	_, err := storage.Exec(ctx, r.db, sql, userID, icenetwork)

	return errors.Wrap(err, "failed to update referred by for all of user's t1 referrals")
}

func (r *repository) deleteUserTracking(ctx context.Context, usr *UserSnapshot) error {
	if usr.Before != nil && usr.User == nil {
		return errors.Wrapf(r.trackingClient.DeleteUser(ctx, usr.Before.ID), "failed to delete tracking data for userID:%v", usr.Before.ID)
	}

	return nil
}
