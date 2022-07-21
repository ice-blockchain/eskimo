// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
)

func (s *service) setupUserValidationRoutes(router *server.Router) {
	router.
		Group("v1w").
		PUT("user-validations/:userId/phone-number", server.RootHandler(s.ValidatePhoneNumber))
}

// ValidatePhoneNumber godoc
// @Schemes
// @Description Validates a provided phone number by a one time code previously provided to the user via SMS.
// @Tags        Validations
// @Accept      json
// @Produce     json
// @Param       Authorization header string                 true "Insert your access token" default(Bearer <Add access token here>)
// @Param       userId        path   string                 true "ID of the user"
// @Param       request       body   ValidatePhoneNumberArg true "Request params"
// @Success     200           "ok"
// @Failure     400           {object} server.ErrorResponse "if validations fail"
// @Failure     401           {object} server.ErrorResponse "if not authorized"
// @Failure     403           {object} server.ErrorResponse "if not allowed"
// @Failure     404           {object} server.ErrorResponse "phone number is not in the process of validation or user not found"
// @Failure     422           {object} server.ErrorResponse "if syntax fails"
// @Failure     500           {object} server.ErrorResponse
// @Failure     504           {object} server.ErrorResponse "if request times out"
// @Router      /user-validations/{userId}/phone-number [PUT].
func (s *service) ValidatePhoneNumber( //nolint:gocritic // False negative.
	ctx context.Context,
	req *server.Request[ValidatePhoneNumberArg, any],
) (*server.Response[any], *server.Response[server.ErrorResponse]) {
	pnv := &users.PhoneNumberValidation{
		UserID:          req.Data.UserID,
		PhoneNumber:     req.Data.PhoneNumber,
		PhoneNumberHash: req.Data.PhoneNumberHash,
		ValidationCode:  req.Data.ValidationCode,
	}
	if err := s.usersProcessor.ValidatePhoneNumber(ctx, pnv); err != nil {
		err = errors.Wrapf(err, "validate phone number failed for %#v", req.Data)
		switch {
		case errors.Is(err, users.ErrRelationNotFound):
			return nil, server.NotFound(err, userNotFoundErrorCode)
		case errors.Is(err, users.ErrNotFound):
			return nil, server.NotFound(err, phoneValidationNotFoundErrorCode)
		case errors.Is(err, users.ErrInvalidPhoneValidationCode):
			return nil, server.BadRequest(err, invalidValidationCodeErrorCode)
		case errors.Is(err, users.ErrExpiredPhoneValidationCode):
			return nil, server.BadRequest(err, phoneValidationCodeExpiredErrorCode)
		}

		return nil, server.Unexpected(err)
	}

	return server.OK[any](), nil
}
