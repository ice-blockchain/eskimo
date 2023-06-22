// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"fmt"
	"math"
	"strings"
	stdlibtime "time"

	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	storage "github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/time"
)

func (r *repository) GetUserGrowth(ctx context.Context, days uint64) (*UserGrowthStatistics, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "context failed")
	}
	const totalAndActiveFactor = 2
	keys := make([]string, 0, totalAndActiveFactor*days+1)
	keys = append(keys, totalUsersGlobalKey)
	now := time.Now()
	for day := stdlibtime.Duration(0); day < stdlibtime.Duration(days); day++ {
		currentDay := now.Add(-1 * day * r.cfg.GlobalAggregationInterval.Parent)
		keys = append(append(keys, r.totalUsersGlobalParentKey(&currentDay)), r.totalActiveUsersGlobalChildrenKeys(&currentDay)...)
	}
	values, err := r.getGlobalValues(ctx, keys...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to getGlobalValues for keys:%#v", keys)
	}

	return r.aggregateGlobalValuesToGrowth(days, now, values, keys), nil
}

//nolint:gocognit,revive,funlen // .
func (r *repository) aggregateGlobalValuesToGrowth(days uint64, now *time.Time, values []*GlobalUnsigned, keys []string) *UserGrowthStatistics {
	nsSinceParentIntervalZeroValue := r.cfg.nanosSinceGlobalAggregationIntervalParentZeroValue(now)
	stats := make([]*UserCountTimeSeriesDataPoint, days, days) //nolint:gosimple // .
	type activeUsersAggregation struct {
		Count  uint64
		Sum    uint64
		DayIdx uint64
	}
	var current activeUsersAggregation
	for ix, key := range keys {
		if ix == 0 {
			continue
		}
		var val uint64
		for _, row := range values {
			if key == row.Key {
				val = row.Value

				break
			}
		}
		if strings.HasPrefix(key, totalUsersGlobalKey) { //nolint:nestif // .
			if current.DayIdx > 0 && current.Count > 0 {
				stats[current.DayIdx-1].UserCount.Active = uint64(math.Round(float64(current.Sum) / float64(current.Count)))
			}
			if stats[current.DayIdx] == nil {
				stats[current.DayIdx] = new(UserCountTimeSeriesDataPoint)
			}
			stats[current.DayIdx].UserCount.Total = val
			if current.DayIdx == 0 && stats[current.DayIdx].Date == nil {
				stats[current.DayIdx].Date = now
			} else if stats[current.DayIdx].Date == nil {
				fullNegativeDayDuration := -1 * r.cfg.GlobalAggregationInterval.Parent * stdlibtime.Duration(current.DayIdx-1)
				stats[current.DayIdx].Date = time.New(now.Add(fullNegativeDayDuration).Add(-nsSinceParentIntervalZeroValue - 1))
			}
			current = activeUsersAggregation{Count: 0, Sum: 0, DayIdx: current.DayIdx + 1}
		} else {
			current.Sum += val
			if val > 0 {
				current.Count++
			}
		}
	}
	stats[current.DayIdx-1].UserCount.Active = uint64(math.Round(float64(current.Sum) / float64(current.Count)))
	stats[0].Total = values[0].Value

	return &UserGrowthStatistics{
		TimeSeries: stats,
		UserCount: UserCount{
			Active: stats[0].Active,
			Total:  values[0].Value,
		},
	}
}

func (r *repository) getGlobalValues(ctx context.Context, keys ...string) ([]*GlobalUnsigned, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "context failed")
	}
	placeholders := make([]string, 0, len(keys))
	params := make([]any, len(keys)+1) //nolint:makezero // .
	params[0] = ""
	for i, key := range keys {
		params[i+1] = key
		placeholders = append(placeholders, fmt.Sprintf("$%v", i+2)) //nolint:gomnd // Not a magic number.
		params[0] = fmt.Sprintf("%v,%v", params[0], key)
	}
	sql := fmt.Sprintf(`SELECT *
						FROM global
						WHERE key in (%v)
						ORDER BY POSITION(key in $1)`, strings.Join(placeholders, ","))
	vals, err := storage.Select[GlobalUnsigned](ctx, r.db, sql, params...)

	return vals, errors.Wrapf(err, "failed to select global vals for keys:%#v", keys)
}

func (r *repository) incrementTotalUsers(ctx context.Context, usr *UserSnapshot) error {
	if usr.Before != nil && usr.Before.ID != "" && usr.User != nil && usr.User.ID != "" {
		return nil
	}

	if usr.Before == nil {
		return r.incrementOrDecrementTotalUsers(ctx, usr.CreatedAt, true)
	}

	return r.incrementOrDecrementTotalUsers(ctx, time.Now(), false)
}

//nolint:revive // .
func (r *repository) incrementOrDecrementTotalUsers(ctx context.Context, date *time.Time, increment bool) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline")
	}
	operation := "+"
	if !increment {
		operation = "-"
	}
	params := []any{totalUsersGlobalKey, r.totalUsersGlobalParentKey(date.Time), r.totalUsersGlobalChildKey(date.Time)}
	sqlParams := make([]string, 0, len(params))
	for idx := range params {
		if idx > 0 {
			sqlParams = append(sqlParams, fmt.Sprintf(
				"($%[1]v,(select GREATEST(total.value %[2]v 1,0) FROM global total WHERE total.key = '%[3]v'))",
				idx+1, operation, params[0]))
		} else {
			sqlParams = append(sqlParams, fmt.Sprintf("($%v,1)", idx+1))
		}
	}
	sql := fmt.Sprintf(`INSERT INTO global (key, value) VALUES %[2]v
								ON CONFLICT (key) DO UPDATE    
						SET value = (select GREATEST(total.value %[1]v 1,0) FROM global total WHERE total.key = '%[3]v')`, operation, strings.Join(sqlParams, ","), params[0])
	if _, err := storage.Exec(ctx, r.db, sql, params...); err != nil && !storage.IsErr(err, storage.ErrNotFound) {
		return errors.Wrapf(err, "failed to update global.value to global.value%v1 of key='%v', for params:%#v ", operation, totalUsersGlobalKey, params)
	}
	keys := make([]string, 0, len(params))
	for _, v := range params {
		keys = append(keys, v.(string)) //nolint:forcetypeassert // We know for sure.
	}

	return errors.Wrapf(r.notifyGlobalValueUpdateMessage(ctx, keys...), "failed to notifyGlobalValueUpdateMessage, keys:%#v", keys)
}

func (r *repository) incrementTotalActiveUsers(ctx context.Context, ms *miningSession) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline")
	}
	keys := ms.detectIncrTotalActiveUsersKeys(r)
	if len(keys) == 0 {
		return nil
	}
	sqlParams := make([]string, 0, len(keys))
	for idx := range keys {
		sqlParams = append(sqlParams, fmt.Sprintf("($%v,1)", idx+1))
	}
	sql := fmt.Sprintf(`
				INSERT INTO global (key, value) VALUES 
					%v
				ON CONFLICT (key) DO UPDATE   
						SET value = global.value + 1`, strings.Join(sqlParams, ","))

	if _, err := storage.Exec(ctx, r.db, sql, keys...); err != nil && !storage.IsErr(err, storage.ErrNotFound) {
		return errors.Wrapf(err, "failed to update global.value to global.value+1 for keys:%#v", keys) //nolint:asasalint // Wrong.
	}

	return nil
}

func (ms *miningSession) detectIncrTotalActiveUsersKeys(repo *repository) []any {
	keys := make([]any, 0)
	start, end := ms.EndedAt.Add(-ms.Extension), *ms.EndedAt.Time
	if !ms.LastNaturalMiningStartedAt.Equal(*ms.StartedAt.Time) ||
		(!ms.PreviouslyEndedAt.IsNil() &&
			repo.totalActiveUsersGlobalChildKey(ms.StartedAt.Time) == repo.totalActiveUsersGlobalChildKey(ms.PreviouslyEndedAt.Time)) {
		start = start.Add(repo.cfg.GlobalAggregationInterval.Child)
	}
	for start.Before(end) {
		keys = append(keys, repo.totalActiveUsersGlobalChildKey(&start))
		start = start.Add(repo.cfg.GlobalAggregationInterval.Child)
	}
	keys = append(keys, repo.totalActiveUsersGlobalChildKey(&end))

	return keys
}

func (r *repository) notifyGlobalValueUpdateMessage(ctx context.Context, keys ...string) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline")
	}
	values, err := r.getGlobalValues(ctx, keys...)
	if err != nil {
		return errors.Wrapf(err, "failed to get global values for keys:%#v", keys)
	}

	return errors.Wrapf(sendMessagesConcurrently(ctx, r.sendGlobalValueMessage, values),
		"failed to sendMessagesConcurrently[sendGlobalValueMessage] for %#v", values)
}

func (r *repository) sendGlobalValueMessage(ctx context.Context, globalVal *GlobalUnsigned) error {
	valueBytes, err := json.MarshalContext(ctx, globalVal)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal %#v", globalVal)
	}

	msg := &messagebroker.Message{
		Headers: map[string]string{"producer": "eskimo"},
		Key:     globalVal.Key,
		Topic:   r.cfg.MessageBroker.Topics[3].Name,
		Value:   valueBytes,
	}

	responder := make(chan error, 1)
	defer close(responder)
	r.mb.SendMessage(ctx, msg, responder)

	return errors.Wrapf(<-responder, "failed to send `%v` message to broker, msg:%#v", msg.Topic, globalVal)
}

func (r *repository) totalUsersGlobalParentKey(date *stdlibtime.Time) string {
	return fmt.Sprintf("%v_%v", totalUsersGlobalKey, date.Format(r.cfg.globalAggregationIntervalParentDateFormat()))
}

func (r *repository) totalUsersGlobalChildKey(date *stdlibtime.Time) string {
	return fmt.Sprintf("%v_%v", totalUsersGlobalKey, date.Format(r.cfg.globalAggregationIntervalChildDateFormat()))
}

func (r *repository) totalActiveUsersGlobalChildKey(date *stdlibtime.Time) string {
	return fmt.Sprintf("%v_%v", totalActiveUsersGlobalKey, date.Format(r.cfg.globalAggregationIntervalChildDateFormat()))
}

func (r *repository) totalActiveUsersGlobalChildrenKeys(date *stdlibtime.Time) []string {
	parent := date.Truncate(r.cfg.GlobalAggregationInterval.Parent)
	current := parent
	keys := make([]string, 0)
	for current.Before(parent.Add(r.cfg.GlobalAggregationInterval.Parent)) {
		keys = append(keys, fmt.Sprintf("%v_%v", totalActiveUsersGlobalKey, current.Format(r.cfg.globalAggregationIntervalChildDateFormat())))
		current = current.Add(r.cfg.GlobalAggregationInterval.Child)
	}

	return keys
}

func NanosSinceMidnight(now *time.Time) stdlibtime.Duration {
	return stdlibtime.Duration(now.Nanosecond()) +
		stdlibtime.Duration(now.Second())*stdlibtime.Second +
		stdlibtime.Duration(now.Minute())*stdlibtime.Minute +
		stdlibtime.Duration(now.Hour())*stdlibtime.Hour
}
