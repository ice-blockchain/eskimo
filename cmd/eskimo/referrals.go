// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
)

func (s *service) setupUserReferralRoutes(router *gin.Engine) {
	router.
		Group("v1r").
		GET("users/:userId/referral-acquisition-history", server.RootHandler(newRequestGetReferralAcquisitionHistory, s.GetReferralAcquisitionHistory)).
		GET("users/:userId/referrals", server.RootHandler(newRequestGetReferrals, s.GetReferrals))
}

// GetReferralAcquisitionHistory godoc
// @Schemes
// @Description  Returns the history of referral acquisition for the provided user id.
// @Tags         Referrals
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string  true   "Insert your access token"  default(Bearer <Add access token here>)
// @Param        userId         path      string  true   "ID of the user"
// @Param        days           query     uint64  false  "The number of days to look in the past. Defaults to 5."
// @Success      200            {array}   users.ReferralAcquisition
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      403            {object}  server.ErrorResponse  "if not allowed"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /users/{userId}/referral-acquisition-history [GET].
func (s *service) GetReferralAcquisitionHistory(ctx context.Context, req *RequestGetReferralAcquisitionHistory) server.Response {
	res, err := s.usersRepository.GetReferralAcquisitionHistory(ctx, &req.GetReferralAcquisitionHistoryArg)
	if err != nil {
		return server.Unexpected(errors.Wrapf(err, "error getting referral acquisition history for %#v", &req.GetReferralAcquisitionHistoryArg))
	}

	return server.OK(res)
}

func newRequestGetReferralAcquisitionHistory() *RequestGetReferralAcquisitionHistory {
	return new(RequestGetReferralAcquisitionHistory)
}

func (req *RequestGetReferralAcquisitionHistory) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestGetReferralAcquisitionHistory) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestGetReferralAcquisitionHistory) Validate() *server.Response {
	if req.Days == 0 {
		req.Days = 5
	}
	if req.AuthenticatedUser.ID != req.UserID {
		return server.Forbidden(errors.Errorf("you can only see your own referral acquisition history. u>%v!=a>%v", req.UserID, req.AuthenticatedUser.ID))
	}

	return server.RequiredStrings(map[string]string{"userId": req.UserID})
}

func (*RequestGetReferralAcquisitionHistory) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindUri, c.ShouldBindQuery, server.ShouldBindAuthenticatedUser(c)}
}

// GetReferrals godoc
// @Schemes
// @Description  Returns the referrals of an user.
// @Tags         Referrals
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string  true   "Insert your access token"  default(Bearer <Add access token here>)
// @Param        userId         path      string  true   "ID of the user"
// @Param        type           query     string  true   "Type of referrals: `CONTACTS` or `T1` or `T2`"
// @Param        limit          query     uint64  false  "Limit of elements to return. Defaults to 10"
// @Param        offset         query     uint64  false  "Number of elements to skip before collecting elements to return"
// @Success      200            {object}  users.Referrals
// @Failure      400            {object}  server.ErrorResponse  "if validations fail"
// @Failure      401            {object}  server.ErrorResponse  "if not authorized"
// @Failure      403            {object}  server.ErrorResponse  "if not allowed"
// @Failure      422            {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500            {object}  server.ErrorResponse
// @Failure      504            {object}  server.ErrorResponse  "if request times out"
// @Router       /users/{userId}/referrals [GET].
func (s *service) GetReferrals(ctx context.Context, req *RequestGetReferrals) server.Response {
	referrals, err := s.usersRepository.GetReferrals(ctx, &req.GetReferralsArg)
	if err != nil {
		return server.Unexpected(errors.Wrapf(err, "failed to get referrals for %#v", &req.GetReferralsArg))
	}

	return server.OK(referrals)
}

func newRequestGetReferrals() *RequestGetReferrals {
	return new(RequestGetReferrals)
}

func (req *RequestGetReferrals) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestGetReferrals) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestGetReferrals) Validate() *server.Response {
	if err := server.RequiredStrings(map[string]string{"userId": req.UserID, "type": req.Type}); err != nil {
		return err
	}
	if req.AuthenticatedUser.ID != req.UserID {
		return server.Forbidden(errors.Errorf("you can only see your own referrals. u>%#v!=a>%v", req.UserID, req.AuthenticatedUser.ID))
	}
	if req.Limit == 0 {
		req.Limit = 10
	}
	req.Type = strings.ToUpper(req.Type)
	for _, referralType := range users.ReferralTypes {
		if req.Type == referralType {
			return nil
		}
	}

	return server.BadRequest(errors.Errorf("type '%v' is invalid, valid types are %v", req.Type, users.ReferralTypes), invalidPropertiesErrorCode)
}

func (*RequestGetReferrals) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindUri, c.ShouldBindQuery, server.ShouldBindAuthenticatedUser(c)}
}
