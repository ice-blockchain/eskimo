// SPDX-License-Identifier: ice License 1.0

package api

import (
	"context"
	"fmt"
	"strings"
	stdlibtime "time"

	"github.com/pkg/errors"

	appcfg "github.com/ice-blockchain/wintr/config"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

func NewClient(ctx context.Context, _ context.CancelFunc) Client {
	var cfg config
	appcfg.MustLoadFromKey(applicationYamlKey, &cfg)
	if cfg.MaxResetCount == nil {
		panic("maxResetCount is not set")
	}

	if cfg.GlobalStartDate == "" {
		panic("globalStartDate is not set")
	}
	globalStartDate, err := stdlibtime.ParseInLocation(stdlibtime.RFC3339Nano, cfg.GlobalStartDate, stdlibtime.UTC)
	log.Panic(err) //nolint:revive // .
	cfg.globalStartDate = time.New(globalStartDate)
	if cfg.AvailabilityWindowSeconds == 0 {
		panic("availabilityWindowSeconds is not set")
	}
	if cfg.MaxAttemptsAllowed == 0 {
		panic("maxAttemptsAllowed is not set")
	}
	c := &client{
		cfg: &cfg,
		db:  storage.MustConnect(ctx, "", applicationYamlKey),
	}

	return c
}

func (c *client) GetQuizStatus(ctx context.Context, userIDs []string) (map[string]*QuizStatus, error) { //nolint:funlen //.
	// $1: global start date.
	// $2: availability window (seconds).
	// $3: max reset count.
	// $4: max attempts allowed.
	// $... UserIDs.
	params, placeholders, res := []any{
		c.cfg.globalStartDate.Time,
		c.cfg.AvailabilityWindowSeconds,
		*c.cfg.MaxResetCount,
		c.cfg.MaxAttemptsAllowed,
	}, make([]string, 0, len(userIDs)), make(map[string]*QuizStatus, len(userIDs))
	if len(userIDs) == 0 {
		return res, nil
	}
	i := len(params)
	for _, userID := range userIDs {
		params = append(params, userID)
		placeholders = append(placeholders, fmt.Sprintf("$%v", i+1))
		i++
	}

	sql := fmt.Sprintf(`SELECT u.id,
    				GREATEST($4 - coalesce(count(fqs.user_id),0),0)			  						                            AS kyc_quiz_remaining_attempts,
				   (qr.user_id IS NOT NULL AND cardinality(qr.resets) > $3) 							                        AS kyc_quiz_disabled,
				   qr.resets  							 									 			                        AS kyc_quiz_reset_at,
				   (qs.user_id IS NOT NULL AND qs.ended_at is not null AND qs.ended_successfully = true)                        AS kyc_quiz_completed,
				   GREATEST(u.created_at,$1)  	  							 							                        AS kyc_quiz_availability_started_at,
				   GREATEST(u.created_at,$1) + (interval '1 second' * $2) 	  							                        AS kyc_quiz_availability_ended_at,
				   ((u.kyc_step_passed >= 2 AND u.kyc_step_blocked = 0) OR (u.kyc_step_passed = 1 AND u.kyc_step_blocked = 2))  AS kyc_quiz_available,
				   (qs.user_id IS NOT NULL AND qs.ended_at IS NULL)			  							                        AS has_unfinished_sessions
			FROM users u
				LEFT JOIN quiz_resets qr 
  					   ON qr.user_id = u.id
				LEFT JOIN quiz_sessions qs
					   ON qs.user_id = u.id
				LEFT JOIN failed_quiz_sessions fqs
					   ON fqs.user_id = u.id
					  AND fqs.started_at >= GREATEST(u.created_at,$1) 
			WHERE u.id in (%v)
			GROUP BY qr.user_id,
					 qs.user_id,
					 u.id`,
		strings.Join(placeholders, ","))
	quizStatuses, err := storage.ExecMany[quizStatus](
		ctx,
		c.db,
		sql,
		params...,
	)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			err = nil
		}

		return res, errors.Wrapf(err, "failed to exec CheckQuizStatus sql for userIDs:%#v", userIDs)
	}
	for _, qs := range quizStatuses {
		res[qs.UserID] = qs.QuizStatus
	}

	return res, nil
}

func (c *client) Close() error {
	return errors.Wrapf(c.db.Close(), "failed to close quiz client")
}

func (c *client) CheckHealth(ctx context.Context) error {
	return errors.Wrap(c.db.Ping(ctx), "[health-check] quiz: failed to ping DB")
}
