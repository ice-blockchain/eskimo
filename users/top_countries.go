// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"fmt"
	"strings"

	"github.com/framey-io/go-tarantool"
	"github.com/pkg/errors"

	devicemetadata "github.com/ice-blockchain/eskimo/users/internal/device/metadata"
)

func (r *repository) GetTopCountries(ctx context.Context, arg *GetTopCountriesArg) (cs []*CountryStatistics, err error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get top countries failed because context failed")
	}
	countries, params := r.getTopCountriesParams(arg)
	sql := fmt.Sprintf(`
						SELECT  country, 
								user_count 
						FROM users_per_country
						WHERE lower(country) in (%v)
						ORDER BY user_count desc 
						LIMIT %v OFFSET :offset`, countries, arg.Limit)
	err = errors.Wrapf(r.db.PrepareExecuteTyped(sql, params, &cs), "get top countries failed for %#v", arg)

	return
}

func (r *repository) getTopCountriesParams(a *GetTopCountriesArg) (countriesSQLEnumeration string, params map[string]interface{}) {
	countriesSQLEnumeration = "''"
	params = map[string]interface{}{
		"offset": a.Offset,
	}
	keyword := strings.ToLower(a.Keyword)
	if keyword == "" {
		countriesSQLEnumeration = "lower(country)"
	} else if countries := r.LookupCountries(keyword); len(countries) != 0 {
		var countryParams []string
		for i, country := range countries {
			k := fmt.Sprintf("country%v", i)
			countryParams = append(countryParams, fmt.Sprintf(":%v", k))
			params[k] = strings.ToLower(country)
		}
		countriesSQLEnumeration = strings.Join(countryParams, ",")
	}

	return
}

func (r *repository) incrementOrDecrementCountryUserCount(ctx context.Context, country devicemetadata.Country, operation arithmeticOperation) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	arOp := []tarantool.Op{{Op: string(operation), Field: 1, Arg: 1}}
	insertTuple := &CountryStatistics{Country: country, UserCount: 1}

	return errors.Wrapf(r.db.UpsertTyped("USERS_PER_COUNTRY", insertTuple, arOp, &[]*CountryStatistics{}),
		"error changing country count for country:%v & operation:%v", country, operation)
}
