// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
)

func (s *service) setupUserStatisticsRoutes(router *server.Router) {
	router.
		Group("v1r").
		GET("user-statistics/top-countries", server.RootHandler(s.GetTopCountries))
}

// GetTopCountries godoc
// @Schemes
// @Description Returns the paginated view of users per country.
// @Tags        Statistics
// @Accept      json
// @Produce     json
// @Param       Authorization header   string true  "Insert your access token" default(Bearer <Add access token here>)
// @Param       keyword       query    string false "a keyword to look for in all country codes or names"
// @Param       limit         query    uint64 false "Limit of elements to return. Defaults to 10"
// @Param       offset        query    uint64 false "Number of elements to skip before collecting elements to return"
// @Success     200           {array}  users.CountryStatistics
// @Failure     400           {object} server.ErrorResponse "if validations fail"
// @Failure     401           {object} server.ErrorResponse "if not authorized"
// @Failure     422           {object} server.ErrorResponse "if syntax fails"
// @Failure     500           {object} server.ErrorResponse
// @Failure     504           {object} server.ErrorResponse "if request times out"
// @Router      /user-statistics/top-countries [GET].
func (s *service) GetTopCountries( //nolint:gocritic // False negative.
	ctx context.Context,
	req *server.Request[GetTopCountriesArg, []*users.CountryStatistics],
) (*server.Response[[]*users.CountryStatistics], *server.Response[server.ErrorResponse]) {
	if req.Data.Limit == 0 {
		req.Data.Limit = 10
	}
	result, err := s.usersRepository.GetTopCountries(ctx, req.Data.Keyword, req.Data.Limit, req.Data.Offset)
	if err != nil {
		return nil, server.Unexpected(errors.Wrapf(err, "failed to get top countries for: %#v", req.Data))
	}

	return server.OK(&result), nil
}
