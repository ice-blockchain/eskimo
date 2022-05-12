// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"

	"github.com/framey-io/go-tarantool"
	"github.com/pkg/errors"
)

func (u *users) GetTopCountries(ctx context.Context, limit Limit, offset Offset) (cs []*CountryStatistics, err error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get top countries failed because context failed")
	}

	var result []*CountryStatistics

	sql := `SELECT country, user_count FROM users_per_country ORDER BY user_count desc LIMIT :limit OFFSET :offset`
	params := map[string]interface{}{"limit": limit, "offset": offset}

	if err = u.db.PrepareExecuteTyped(sql, params, &result); err != nil {
		return nil, errors.Wrap(err, "get top countries failed")
	}

	return result, nil
}

func (mb *usersSource) incrementOrDecrementCountryUserCount(ctx context.Context, country string, operation arithmeticOperation) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}

	arOp := []tarantool.Op{{Op: string(operation), Field: 1, Arg: 1}}
	insertTuple := &usersPerCountry{Country: country, UserCount: 1}

	err := mb.db.UpsertAsync("USERS_PER_COUNTRY", insertTuple, arOp).GetTyped(&[]usersPerCountry{})

	return errors.Wrap(err, "error changing country count")
}
