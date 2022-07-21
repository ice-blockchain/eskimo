// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
)

func (s *service) setupDevicesRoutes(router *server.Router) {
	router.
		Group("v1r").
		GET("users/:userId/devices/:deviceUniqueId/settings", server.RootHandler(s.GetDeviceSettings))
}

// GetDeviceSettings godoc
// @Schemes
// @Description Returns the settings of an user's device
// @Tags        Devices
// @Accept      json
// @Produce     json
// @Param       Authorization  header   string true "Insert your access token" default(Bearer <Add access token here>)
// @Param       userId         path     string true "ID of the user"
// @Param       deviceUniqueId path     string true "ID of the device"
// @Success     200            {object} users.DeviceSettings
// @Failure     400            {object} server.ErrorResponse "if validations fail"
// @Failure     401            {object} server.ErrorResponse "if not authorized"
// @Failure     403            {object} server.ErrorResponse "if not allowed"
// @Failure     404            {object} server.ErrorResponse "if not found"
// @Failure     422            {object} server.ErrorResponse "if syntax fails"
// @Failure     500            {object} server.ErrorResponse
// @Failure     504            {object} server.ErrorResponse "if request times out"
// @Router      /users/{userId}/devices/{deviceUniqueId}/settings [GET].
func (s *service) GetDeviceSettings( //nolint:gocritic // False negative.
	ctx context.Context,
	req *server.Request[GetDeviceSettingsArg, users.DeviceSettings],
) (*server.Response[users.DeviceSettings], *server.Response[server.ErrorResponse]) {
	resp, err := s.usersRepository.GetDeviceSettings(ctx, users.DeviceID{UserID: req.Data.UserID, DeviceUniqueID: req.Data.DeviceUniqueID})
	if err != nil {
		err = errors.Wrapf(err, "failed to GetDeviceSettings for %#v", req.Data)
		switch {
		case errors.Is(err, users.ErrNotFound):
			return nil, server.NotFound(err, deviceSettingsNotFoundErrorCode)
		default:
			return nil, server.Unexpected(err)
		}
	}

	return server.OK(resp), nil
}
