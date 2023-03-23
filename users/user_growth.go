// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	stdlibtime "time"

	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/time"
)

func (r *repository) GetUserGrowth(ctx context.Context, days uint64) (*UserGrowthStatistics, error) { //nolint:funlen,gocognit,revive // Alot of mappings.
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "context failed")
	}
	const totalAndActiveFactor = 2
	keys := make([]string, 0, totalAndActiveFactor*days+1)
	keys = append(keys, totalUsersGlobalKey)
	now := time.Now()
	for day := stdlibtime.Duration(0); day < stdlibtime.Duration(days); day++ {
		currentDay := now.Add(-1 * day * r.cfg.GlobalAggregationInterval.Parent)
		keys = append(keys, r.totalActiveUsersGlobalParentKey(&currentDay), r.totalUsersGlobalParentKey(&currentDay))
	}
	values, err := r.getGlobalValues(ctx, keys...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to getGlobalValues for keys:%#v", keys)
	}
	nsSinceParentIntervalZeroValue := r.cfg.nanosSinceGlobalAggregationIntervalParentZeroValue(now)
	stats := make([]*UserCountTimeSeriesDataPoint, days, days) //nolint:gosimple // .
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
		dayIdx := (ix - 1) / totalAndActiveFactor
		if stats[dayIdx] == nil {
			stats[dayIdx] = new(UserCountTimeSeriesDataPoint)
		}
		if dayIdx == 0 && stats[dayIdx].Date == nil {
			stats[dayIdx].Date = now
		} else if stats[dayIdx].Date == nil {
			fullNegativeDayDuration := -1 * r.cfg.GlobalAggregationInterval.Parent * stdlibtime.Duration(dayIdx-1)
			stats[dayIdx].Date = time.New(now.Add(fullNegativeDayDuration).Add(-nsSinceParentIntervalZeroValue - 1))
		}
		if (ix-1)%totalAndActiveFactor == 0 {
			stats[dayIdx].Active = val
		} else {
			stats[dayIdx].Total = val
		}
	}

	return &UserGrowthStatistics{
		TimeSeries: stats,
		UserCount: UserCount{
			Active: stats[0].Active,
			Total:  values[0].Value,
		},
	}, nil
}

func (r *repository) getGlobalValues(ctx context.Context, keys ...string) ([]*GlobalUnsigned, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "context failed")
	}
	values := make([]string, 0, len(keys))
	params := make(map[string]any, len(keys)+1)
	for i, key := range keys {
		params[fmt.Sprintf("key%v", i)] = key
		values = append(values, fmt.Sprintf(":key%v", i))
		if params["positions"] == nil {
			params["positions"] = key
		} else {
			params["positions"] = fmt.Sprintf("%v,%v", params["positions"], key)
		}
	}
	sql := fmt.Sprintf(`SELECT *
						FROM global
						WHERE key in (%v)
						ORDER BY POSITION(key, :positions)`, strings.Join(values, ","))
	vals := make([]*GlobalUnsigned, 0, len(keys))
	err := r.db.PrepareExecuteTyped(sql, params, &vals)

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

//nolint:funlen,gocognit,gocyclo,revive,cyclop // .
func (r *repository) incrementOrDecrementTotalUsers(ctx context.Context, date *time.Time, increment bool) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline")
	}
	operation := "+"
	if !increment {
		operation = "-"
	}
	params := make(map[string]any, 1+1+1)
	params["total_key"] = totalUsersGlobalKey
	params["total_parent_key"] = r.totalUsersGlobalParentKey(date.Time)
	params["total_child_key"] = r.totalUsersGlobalChildKey(date.Time)
	sqlParams := make([]string, 0, len(params))
	for k := range params {
		sqlParams = append(sqlParams, fmt.Sprintf(":%s", k))
	}
	sql := fmt.Sprintf(`UPDATE global  
						SET value = (select CAST(GREATEST(CAST(total.value AS UNSIGNED) %[1]v 1,0) AS UNSIGNED) FROM global total WHERE total.key = :total_key) 
						WHERE key in (%[2]v)`, operation, strings.Join(sqlParams, ","))
	resp, err := r.db.PrepareExecute(sql, params)
	if err = storage.CheckSQLDMLErr(resp, err); err != nil && !errors.Is(err, storage.ErrNotFound) {
		return errors.Wrapf(err, "failed to update global.value to global.value%v1 of key='%v', for params:%#v ", operation, totalUsersGlobalKey, params)
	}
	updatedRows, err := strconv.Atoi(fmt.Sprint(resp.Data[0]))
	if err != nil {
		return errors.Wrapf(err, "not a number")
	}
	if updatedRows < len(params) { //nolint:nestif // .
		for key, val := range params {
			if val == totalUsersGlobalKey {
				continue
			}
			sql = fmt.Sprintf(`INSERT INTO global (key,value) 
							   SELECT :%v,
									  CAST(value AS UNSIGNED)
							   FROM global  
							   WHERE key = :total_key`, key)
			innerParams := map[string]any{key: val, "total_key": params["total_key"]}
			if err = storage.CheckSQLDMLErr(r.db.PrepareExecute(sql, innerParams)); err != nil && !errors.Is(err, storage.ErrDuplicate) {
				return errors.Wrapf(err, "failed to insert global.value from existing global.value of key='%v', for k:%v,v:%v ", totalUsersGlobalKey, key, val)
			}
			if err != nil && errors.Is(err, storage.ErrDuplicate) {
				sql = fmt.Sprintf(`UPDATE global  
								   SET value = (select CAST(total.value AS UNSIGNED) FROM global total WHERE total.key = :total_key) 
								   WHERE key = :%v`, key)
				if err = storage.CheckSQLDMLErr(r.db.PrepareExecute(sql, innerParams)); err != nil {
					return errors.Wrapf(err, "failed to update global.value to existing global.value of key='%v', for k:%v,v:%v ", totalUsersGlobalKey, key, val)
				}
			}
		}
	}
	keys := make([]string, 0, len(params))
	for _, v := range params {
		keys = append(keys, v.(string)) //nolint:forcetypeassert // We know for sure.
	}

	return errors.Wrapf(r.notifyGlobalValueUpdateMessage(ctx, keys...), "failed to notifyGlobalValueUpdateMessage, keys:%#v", keys)
}

func (r *repository) incrementTotalActiveUsers(ctx context.Context, prev, next *time.Time) error { //nolint:funlen,gocognit,gocyclo,revive,cyclop // .
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline")
	}
	parent, child := r.totalActiveUsersGlobalParentKey(next.Time), r.totalActiveUsersGlobalChildKey(next.Time)
	skipParent := prev != nil && r.totalActiveUsersGlobalParentKey(prev.Time) == parent
	skipChild := prev != nil && r.totalActiveUsersGlobalChildKey(prev.Time) == child
	if skipChild && skipParent {
		return nil
	}
	params := make(map[string]any, 1+1)
	if !skipParent {
		params["total_per_parent_duration_key"] = parent
	}
	if !skipChild {
		params["total_per_child_key"] = child
	}
	sqlParams := make([]string, 0, len(params))
	for k := range params {
		sqlParams = append(sqlParams, fmt.Sprintf(":%s", k))
	}
	sql := fmt.Sprintf(`UPDATE global  
						SET value = CAST(CAST(value AS UNSIGNED)+1 AS UNSIGNED)
						WHERE key in (%v)`, strings.Join(sqlParams, ","))
	resp, err := r.db.PrepareExecute(sql, params)
	if err = storage.CheckSQLDMLErr(resp, err); err != nil && !errors.Is(err, storage.ErrNotFound) {
		return errors.Wrapf(err, "failed to update global.value to global.value+1 for params:%#v ", params)
	}
	updatedRows, err := strconv.Atoi(fmt.Sprint(resp.Data[0]))
	if err != nil {
		return errors.Wrapf(err, "not a number")
	}
	if updatedRows < len(params) {
		for key, val := range params {
			sql = fmt.Sprintf(`INSERT INTO global (key,value) VALUES (:%v,CAST(1 AS UNSIGNED))`, key)
			innerParams := map[string]any{key: val}
			if err = storage.CheckSQLDMLErr(r.db.PrepareExecute(sql, innerParams)); err != nil && !errors.Is(err, storage.ErrDuplicate) {
				return errors.Wrapf(err, "failed to insert global.value to 1 for params[%v]:%v ", key, val)
			}
			if err != nil && errors.Is(err, storage.ErrDuplicate) {
				sql = fmt.Sprintf(`UPDATE global
								   SET value = CAST(CAST(value AS UNSIGNED)+1 AS UNSIGNED)
								   WHERE key = :%v`, key)
				if err = storage.CheckSQLDMLErr(r.db.PrepareExecute(sql, innerParams)); err != nil {
					return errors.Wrapf(err, "failed to update global.value to global.value+1 for params[%v]:%v ", key, val)
				}
			}
		}
	}
	keys := make([]string, 0, len(params))
	for _, v := range params {
		keys = append(keys, v.(string)) //nolint:forcetypeassert // We know for sure.
	}

	return errors.Wrapf(r.notifyGlobalValueUpdateMessage(ctx, keys...), "failed to notifyGlobalValueUpdateMessage, keys:%#v", keys)
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

func (r *repository) totalActiveUsersGlobalParentKey(date *stdlibtime.Time) string {
	return fmt.Sprintf("%v_%v", totalActiveUsersGlobalKey, date.Format(r.cfg.globalAggregationIntervalParentDateFormat()))
}

func (r *repository) totalUsersGlobalChildKey(date *stdlibtime.Time) string {
	return fmt.Sprintf("%v_%v", totalUsersGlobalKey, date.Format(r.cfg.globalAggregationIntervalChildDateFormat()))
}

func (r *repository) totalActiveUsersGlobalChildKey(date *stdlibtime.Time) string {
	return fmt.Sprintf("%v_%v", totalActiveUsersGlobalKey, date.Format(r.cfg.globalAggregationIntervalChildDateFormat()))
}

func NanosSinceMidnight(now *time.Time) stdlibtime.Duration {
	return stdlibtime.Duration(now.Nanosecond()) +
		stdlibtime.Duration(now.Second())*stdlibtime.Second +
		stdlibtime.Duration(now.Minute())*stdlibtime.Minute +
		stdlibtime.Duration(now.Hour())*stdlibtime.Hour
}
