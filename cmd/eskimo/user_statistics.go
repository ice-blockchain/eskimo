// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"github.com/ICE-Blockchain/eskimo/users"
	"github.com/ICE-Blockchain/wintr/server"
	"github.com/gin-gonic/gin"
)

func (s *service) setupUserStatisticsRoutes(router *gin.Engine) {
	router.
		Group("/v1").
		GET("user-statistics/top-countries", server.RootHandler(newRequestGetTopCountries, s.GetTopCountries))
}

// GetTopCountries godoc
// @Schemes
// @Description  Returns the paginated view of users per country.
// @Tags         Statistics
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string  true   "Insert your access token"  default(Bearer <Add access token here>)
// @Param        limit          query     uint64  false  "Limit of elements to return. Defaults to 20"
// @Param        offset         query     uint64  false  "Number of elements to skip before collecting elements to return"
// @Success      200            {array}   users.CountryStatistics
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /user-statistics/top-countries [GET].
func (s *service) GetTopCountries(ctx context.Context, r server.ParsedRequest) server.Response {
	//req := r.(*RequestGetTopCountries)

	return server.OK([]*users.CountryStatistics{{
		UserCount: 111,
		Country:   "us",
	}})
}

func newRequestGetTopCountries() server.ParsedRequest {
	return new(RequestGetTopCountries)
}

func (req *RequestGetTopCountries) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestGetTopCountries) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestGetTopCountries) Validate() *server.Response {
	if req.Limit == 0 {
		req.Limit = 20
	}

	return nil
}

func (req *RequestGetTopCountries) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{server.ShouldBindAuthenticatedUser(c)}
}
