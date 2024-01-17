// SPDX-License-Identifier: ice License 1.0

package quiz

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	stdlibtime "time"

	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

const (
	alertFrequency = 5 * stdlibtime.Minute
)

func (r *repositoryImpl) startAlerter(ctx context.Context) {
	if !r.config.EnableAlerts {
		log.Info("unsuccessfulKYCSteps alerts not enabled")

		return
	} else if r.config.AlertSlackWebhook == "" || r.config.Environment == "" {
		log.Panic("`alert-slack-webhook` is missing")
	}
	ticker := stdlibtime.NewTicker(*r.config.alertFrequency.Load())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			const deadline = stdlibtime.Minute
			reqCtx, cancel := context.WithTimeout(ctx, deadline)
			log.Error(errors.Wrap(r.sendAlertToSlack(reqCtx, ticker), "failed to sendAlertToSlack"))
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
func (r *repositoryImpl) sendAlertToSlack(ctx context.Context, ticker *stdlibtime.Ticker) error {
	var stats []*quizStats
	if err := storage.DoInTransaction(ctx, r.DB, func(conn storage.QueryExecer) error {
		sql := `SELECT last_alert_at, 
					   frequency_in_seconds 
				FROM quiz_alerts 
				WHERE pk = 1
				FOR UPDATE`
		alert, err := storage.Get[struct {
			LastAlertAt        *time.Time `db:"last_alert_at"`
			FrequencyInSeconds uint64     `db:"frequency_in_seconds"`
		}](ctx, conn, sql)
		if err != nil {
			return errors.Wrap(err, "failed to lock quiz_alerts")
		}
		if time.Now().Sub(*alert.LastAlertAt.Time) < stdlibtime.Duration(float64(r.config.alertFrequency.Load().Nanoseconds())*0.8) { //nolint:gomnd // .
			return errRaceCondition
		}
		if newFrequency := stdlibtime.Duration(alert.FrequencyInSeconds) * stdlibtime.Second; newFrequency != *r.config.alertFrequency.Load() {
			r.config.alertFrequency.Store(&newFrequency)
			ticker.Reset(newFrequency)
		}

		sql = `SELECT (CASE
						WHEN skipped 
							THEN 'skipped' 
						WHEN EXTRACT(EPOCH FROM (ended_at - started_at))  >= $2 
							THEN 'expired'
						WHEN cardinality(answers) < $3
						    THEN 'failed'
						ELSE 'unknown'
					   END)	   AS mapped_reason,
					  count(1) AS counter
			   FROM failed_quiz_sessions
			   WHERE ended_at >= $1
               GROUP BY 1

			   UNION ALL 

			   SELECT 'success' AS mapped_reason,
					  count(1) AS counter
			   FROM quiz_sessions
			   WHERE ended_successfully IS TRUE 
				 AND ended_at IS NOT NULL
				 AND ended_at >= $1`
		stats, err = storage.Select[quizStats](ctx, conn, sql, alert.LastAlertAt.Time, r.config.MaxSessionDurationSeconds, r.config.MaxQuestionsPerSession-r.config.MaxWrongAnswersPerSession) //nolint:lll // .
		if err != nil {
			return errors.Wrap(err, "failed to select stats")
		}

		sql = `UPDATE quiz_alerts 
			   SET last_alert_at = $1
			   WHERE pk = 1`
		updatedRows, err := storage.Exec(ctx, conn, sql, time.Now().Time)
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

	return errors.Wrap(r.sendSlackMessage(sendMsgCtx, stats), "failed to sendSlackMessage") //nolint:contextcheck // .
}

type (
	quizStats struct {
		Reason  string `db:"mapped_reason" json:"reason"`
		Counter uint64 `db:"counter" json:"counter"`
	}
)

//nolint:funlen // .
func (r *repositoryImpl) sendSlackMessage(ctx context.Context, stats []*quizStats) error {
	if len(stats) == 0 {
		return nil
	}
	rows := make([]string, 0, len(stats))
	var hasFailures bool
	for _, stat := range stats {
		if stat.Reason != "success" && stat.Counter > 0 {
			hasFailures = true
		}
		if stat.Counter == 0 {
			continue
		}
		rows = append(rows, fmt.Sprintf("`%v`: `%v`", stat.Reason, stat.Counter))
	}
	if !hasFailures || len(rows) == 0 {
		return nil
	}
	message := struct {
		Text string `json:"text,omitempty"`
	}{
		Text: fmt.Sprintf("[%v]kycStep [4], stats:\n%v", r.config.Environment, strings.Join(rows, "\n")),
	}
	data, err := json.Marshal(message)
	if err != nil {
		return errors.Wrapf(err, "failed to Marshal slack message:%#v", message)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.config.AlertSlackWebhook, bytes.NewBuffer(data))
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
