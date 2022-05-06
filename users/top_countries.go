// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"

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
