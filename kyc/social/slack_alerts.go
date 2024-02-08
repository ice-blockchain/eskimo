// SPDX-License-Identifier: ice License 1.0

package social

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	stdlibtime "time"

	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

const (
	alertFrequency = 5 * stdlibtime.Minute
)

func (r *repository) startUnsuccessfulKYCStepsAlerter(ctx context.Context, kycStep users.KYCStep) {
	if !r.cfg.EnableAlerts {
		log.Info("unsuccessfulKYCSteps alerts not enabled")

		return
	} else if r.cfg.AlertSlackWebhook == "" || r.cfg.Environment == "" {
		log.Panic("`alert-slack-webhook` is missing")
	}
	alertFrequencyDuration, found := r.cfg.alertFrequency.Load(kycStep)
	if !found {
		log.Panic(fmt.Sprintf("failed to get alertFrequency for %v", kycStep))
	}
	ticker := stdlibtime.NewTicker(alertFrequencyDuration.(stdlibtime.Duration)) //nolint:forcetypeassert // .
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			const deadline = stdlibtime.Minute
			reqCtx, cancel := context.WithTimeout(ctx, deadline)
			log.Error(errors.Wrapf(r.sendUnsuccessfulKYCStepsAlertToSlack(reqCtx, ticker, kycStep, TwitterType),
				"failed to sendUnsuccessfulKYCStepsAlertToSlack[%v][%v]", kycStep, TwitterType))
			cancel()
		case <-ctx.Done():
			return
		}
	}
}

// .
var (
	errRaceCondition = errors.New("race condition")
)

//nolint:funlen // .
func (r *repository) sendUnsuccessfulKYCStepsAlertToSlack(ctx context.Context, ticker *stdlibtime.Ticker, kycStep users.KYCStep, social Type) error {
	var stats []*unsuccessfulSocialKYCStats
	if err := storage.DoInTransaction(ctx, r.db, func(conn storage.QueryExecer) error {
		sql := `SELECT last_alert_at, 
					   frequency_in_seconds 
				FROM unsuccessful_social_kyc_alerts 
				WHERE kyc_step = $1
				  AND social = $2
				FOR UPDATE`
		alert, err := storage.Get[struct {
			LastAlertAt        *time.Time `db:"last_alert_at"`
			FrequencyInSeconds uint64     `db:"frequency_in_seconds"`
		}](ctx, conn, sql, kycStep, social)
		if err != nil {
			return errors.Wrap(err, "failed to lock unsuccessful_social_kyc_alerts")
		}
		freq, found := r.cfg.alertFrequency.Load(kycStep)
		if !found {
			log.Panic(fmt.Sprintf("failed to get alertFrequency for %v", kycStep))
		}
		if time.Now().Sub(*alert.LastAlertAt.Time) < stdlibtime.Duration(float64(freq.(stdlibtime.Duration).Nanoseconds())*0.8) { //nolint:gomnd,forcetypeassert,lll // .
			return errRaceCondition
		}
		if newFrequency := stdlibtime.Duration(alert.FrequencyInSeconds) * stdlibtime.Second; newFrequency != freq.(stdlibtime.Duration) { //nolint:forcetypeassert,lll // .
			r.cfg.alertFrequency.Store(kycStep, newFrequency)
			ticker.Reset(newFrequency)
		}

		sql = `SELECT (CASE
							WHEN reason like 'duplicate userhandle %' THEN 'duplicate userhandle'
							WHEN reason like 'duplicate socials %' THEN 'duplicate socials'
							WHEN reason like '%: %' THEN substring(reason from position(': ' in reason) + 2)
							ELSE reason
						END)    AS mapped_reason,
					   count(1) AS counter
				FROM social_kyc_unsuccessful_attempts
				WHERE kyc_step = $1
				  AND social = $2
				  AND created_at >= $3
				GROUP BY mapped_reason
			   UNION ALL 
			   SELECT 'success' AS mapped_reason,
					  count(1) AS counter
				FROM social_kyc_steps
				WHERE kyc_step = $1
				  AND social = $2
				  AND created_at >= $3`
		stats, err = storage.Select[unsuccessfulSocialKYCStats](ctx, conn, sql, kycStep, social, alert.LastAlertAt.Time)
		if err != nil {
			return errors.Wrap(err, "failed to select stats")
		}

		sql = `UPDATE unsuccessful_social_kyc_alerts 
			   SET last_alert_at = $3
			   WHERE kyc_step = $1
				 AND social = $2`
		updatedRows, err := storage.Exec(ctx, conn, sql, kycStep, social, time.Now().Time)
		if err != nil {
			return errors.Wrap(err, "update last_alert_at to now failed")
		}
		if updatedRows == 0 {
			return errors.New("unexpected 0 updatedRows")
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "doInTransaction failed")
	}

	sendMsgCtx, cancel := context.WithTimeout(context.Background(), requestDeadline)
	defer cancel()

	return errors.Wrap(r.sendSlackMessage(sendMsgCtx, kycStep, social, stats), "failed to sendSlackMessage") //nolint:contextcheck // .
}

type (
	unsuccessfulSocialKYCStats struct {
		Reason  string `db:"mapped_reason" json:"reason"`
		Counter uint64 `db:"counter" json:"counter"`
	}
)

//nolint:funlen // .
func (r *repository) sendSlackMessage(ctx context.Context, kycStep users.KYCStep, social Type, stats []*unsuccessfulSocialKYCStats) error {
	if len(stats) == 0 {
		return nil
	}
	rows := make([]string, 0, len(stats))
	var hasExhaustedRetries bool
	for _, stat := range stats {
		if stat.Reason == exhaustedRetriesReason && stat.Counter > 0 {
			hasExhaustedRetries = true
		}
		rows = append(rows, fmt.Sprintf("`%v`: `%v`", stat.Reason, stat.Counter))
	}
	if !hasExhaustedRetries || len(rows) == 0 {
		return nil
	}
	message := struct {
		Text string `json:"text,omitempty"`
	}{
		Text: fmt.Sprintf("[%v]Unsuccessful kycStep [%v], social [%v] stats:\n%v", r.cfg.Environment, kycStep, social, strings.Join(rows, "\n")),
	}
	data, err := json.Marshal(message)
	if err != nil {
		return errors.Wrapf(err, "failed to Marshal slack message:%#v", message)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.cfg.AlertSlackWebhook, bytes.NewBuffer(data))
	if err != nil {
		return errors.Wrap(err, "newRequestWithContext failed")
	}

	resp, err := new(http.Client).Do(req)
	if err != nil {
		return errors.Wrap(err, "slack webhook request failed")
	}
	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("unexpected statusCode:%v", resp.StatusCode)
	}

	return errors.Wrap(resp.Body.Close(), "failed to close body")
}
