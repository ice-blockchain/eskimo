// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	stdlibtime "time"

	"github.com/goccy/go-json"
	"github.com/hashicorp/go-multierror"
	"github.com/imroc/req/v3"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/log"
)

func (r *repository) TryResetKYCSteps(ctx context.Context, userID string) (*User, error) {
	sql := `SELECT r.kyc_steps_to_reset,
				   u.*
			FROM users u
				LEFT JOIN kyc_steps_reset_requests r
					   ON r.user_id = u.id
			WHERE u.id = $1`
	if resp, err := storage.ExecOne[struct {
		KYCStepsToReset []KYCStep `db:"kyc_steps_to_reset"`
		User
	}](ctx, r.db, sql, userID); err != nil {
		return nil, errors.Wrapf(err, "failed to get kyc_steps_reset_requests for userID:%v", userID)
	} else if len(resp.KYCStepsToReset) == 0 {
		r.sanitizeUser(&resp.User)
		r.sanitizeUserForUI(&resp.User)

		return &resp.User, nil
	} else if err = r.resetKYCSteps(ctx, userID, resp.KYCStepsToReset); err != nil {
		return nil, errors.Wrapf(err, "failed to resetKYCSteps for userID:%v", userID)
	}

	return r.TryResetKYCSteps(ctx, userID)
}

func (r *repository) resetKYCSteps(ctx context.Context, userID string, kycStepsToBeReset []KYCStep) error {
	kycStepResetPipelines := make(map[KYCStep]struct{}, len(kycStepsToBeReset))
	for _, kycStep := range kycStepsToBeReset {
		if kycStep == LivenessDetectionKYCStep || kycStep == FacialRecognitionKYCStep {
			kycStepResetPipelines[FacialRecognitionKYCStep] = struct{}{}
		} else {
			kycStepResetPipelines[kycStep] = struct{}{}
		}
	}
	wg := new(sync.WaitGroup)
	wg.Add(len(kycStepResetPipelines))
	errs := make(chan error, len(kycStepResetPipelines))
	for kycStep := range kycStepResetPipelines {
		go func(step KYCStep) {
			defer wg.Done()
			errs <- errors.Wrapf(r.resetKYCStep(ctx, userID, step), "failed to resetKYCStep(%v) for userID:%v", step, userID)
		}(kycStep)
	}
	wg.Wait()
	close(errs)
	responses := make([]error, 0, cap(errs))
	for err := range errs {
		responses = append(responses, err)
	}
	if err := multierror.Append(nil, responses...).ErrorOrNil(); err != nil {
		return errors.Wrapf(err, "atleast one resetKYCStep failed for userID:%v", userID)
	}
	_, err := storage.Exec(ctx, r.db, `DELETE FROM kyc_steps_reset_requests WHERE user_id = $1`, userID)

	return errors.Wrapf(err, "failed to delete kyc step reset request for userID:%v", userID)
}

func (r *repository) resetKYCStep(ctx context.Context, userID string, step KYCStep) error {
	switch step { //nolint:exhaustive // Not needed yet.
	case FacialRecognitionKYCStep:
		if err := r.resetFacialRecognitionKYCStep(ctx, userID); err != nil {
			return errors.Wrapf(err, "failed to resetFacialRecognitionKYCStep for userID:%v", userID)
		}
	default:
		return nil
	}

	sql := `UPDATE kyc_steps_reset_requests 
			SET kyc_steps_to_reset = array_remove(kyc_steps_to_reset, $2::smallint)
			WHERE user_id = $1`
	if updated, err := storage.Exec(ctx, r.db, sql, userID, step); err != nil || updated == 0 {
		if updated == 0 {
			err = errors.Wrapf(ErrNotFound, "failed to remove step[%v] from kyc_steps_reset_requests for userID:%v", step, userID)
		}
		if storage.IsErr(err, storage.ErrCheckFailed) {
			// This happens if the resulting array is empty, at which point we need to delete the entire entry,
			// but we're not going to do that here, cuz it's going to happen anyway in r.resetKYCSteps.
			err = nil
		}
		if err != nil {
			return errors.Wrapf(err, "[db]failed to resetKYCStep[%v] for userID:%v", step, userID)
		}
	}

	return nil
}

func init() { //nolint:gochecknoinits // It's the only way to tweak the client.
	req.DefaultClient().SetJsonMarshal(json.Marshal)
	req.DefaultClient().SetJsonUnmarshal(json.Unmarshal)
	req.DefaultClient().GetClient().Timeout = requestDeadline
}

//nolint:gomnd // Specific config.
func (r *repository) resetFacialRecognitionKYCStep(ctx context.Context, userID string) error {
	if resp, err := req.
		SetContext(ctx).
		SetRetryCount(25).
		SetRetryBackoffInterval(10*stdlibtime.Millisecond, 1*stdlibtime.Second).
		SetRetryHook(func(resp *req.Response, err error) {
			if err != nil {
				log.Error(errors.Wrap(err, "failed to delete face auth state for user, retrying... "))
			} else {
				body, bErr := resp.ToString()
				log.Error(errors.Wrapf(bErr, "failed to parse negative response body for delete face auth state for user"))
				log.Error(errors.Errorf("failed to delete face auth state for user with status code:%v, body:%v, retrying... ", resp.GetStatusCode(), body))
			}
		}).
		SetRetryCondition(func(resp *req.Response, err error) bool {
			return err != nil || (resp.GetStatusCode() != http.StatusOK && resp.GetStatusCode() != http.StatusNoContent && resp.GetStatusCode() != http.StatusUnauthorized) //nolint:lll // .
		}).
		AddQueryParam("caller", "eskimo-hut").
		SetHeader("Authorization", authorization(ctx)).
		Delete(fmt.Sprintf("%v/users/%v", r.cfg.KYC.KYCStep1ResetURL, userID)); err != nil {
		return errors.Wrapf(err, "failed to delete face auth state for userID:%v", userID)
	} else if _, err2 := resp.ToBytes(); err2 != nil {
		return errors.Wrapf(err2, "failed to read body of delete face auth state request for userID:%v", userID)
	} else { //nolint:revive // .
		return nil
	}
}
