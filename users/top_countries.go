// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	storagev2 "github.com/ice-blockchain/wintr/connectors/storage/v2"
)

func (r *repository) GetTopCountries(ctx context.Context, keyword string, limit, offset uint64) (cs []*CountryStatistics, err error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "get top countries failed because context failed")
	}
	countries, countryParams := r.getTopCountriesParams(keyword)
	params := []any{limit, offset, countryParams}
	sql := fmt.Sprintf(`
						SELECT  country, 
								user_count 
						FROM users_per_country
						WHERE lower(country) in (%v)
						ORDER BY user_count desc 
						LIMIT $1 OFFSET $2`, countries)
	cs, err = storagev2.Select[CountryStatistics](ctx, r.dbV2, sql, params...)
	if err != nil {
		return nil, errors.Wrapf(err, "get top countries failed for %v %v %v", keyword, limit, offset)
	}

	return cs, nil
}

func (r *repository) getTopCountriesParams(countryKeyword string) (countriesSQLEnumeration string, params []any) {
	countriesSQLEnumeration = "''"
	params = make([]any, 0)
	keyword := strings.ToLower(countryKeyword)
	if keyword == "" {
		countriesSQLEnumeration = "lower(country)"
	} else if countries := r.LookupCountries(keyword); len(countries) != 0 {
		var countryParams []string
		for i, country := range countries {
			countryParams = append(countryParams, fmt.Sprintf("$%v", i))
			params = append(params, strings.ToLower(country))
		}
		countriesSQLEnumeration = strings.Join(countryParams, ",")
	}

	return
}

func (r *repository) incrementOrDecrementCountryUserCount(ctx context.Context, usr *UserSnapshot) error { //nolint:gocognit // .
	if (usr.User != nil && usr.Before != nil && usr.User.Country == usr.Before.Country) || ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	sqlTemplate := `
INSERT INTO users_per_country (country, user_count) 
VALUES ($1, 1) ON CONFLICT (country) DO UPDATE
	SET user_count = users_per_country.user_count %v 1
`
	return errors.Wrapf(storagev2.DoInTransaction(ctx, r.dbV2, func(conn storagev2.QueryExecer) error {
		if usr.User != nil {
			if _, err := storagev2.Exec(ctx, r.dbV2, fmt.Sprintf(sqlTemplate, "+"), usr.User.Country); err != nil {
				return errors.Wrapf(err, "error increasing country count for country:%v", usr.User.Country)
			}
		}
		if usr.Before != nil {
			if _, err := storagev2.Exec(ctx, r.dbV2, fmt.Sprintf(sqlTemplate, "-"), usr.Before.Country); err != nil {
				return errors.Wrapf(err, "error decreasing country count for country:%v", usr.Before.Country)
			}
		}

		return nil
	}), "failed to execute transaction updating users_per_countries")
}
