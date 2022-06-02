// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users"
	"github.com/ice-blockchain/wintr/server"
)

func (s *service) setupDevicesRoutes(router *gin.Engine) {
	router.
		Group("v1").
		GET("users/:userId/devices/:deviceUniqueId/settings", server.RootHandler(newRequestGetDeviceSettings, s.GetDeviceSettings))
}

// GetDeviceSettings godoc
// @Schemes
// @Description  Returns the settings of an user's device
// @Tags         Devices
// @Accept       json
// @Produce      json
// @Param        Authorization   header    string  true  "Insert your access token"  default(Bearer <Add access token here>)
// @Param        userId          path      string  true  "ID of the user"
// @Param        deviceUniqueId  path      string  true  "ID of the device"
// @Success      200             {object}  users.DeviceSettings
// @Failure      400             {object}  server.ErrorResponse  "if validations fail"
// @Failure      401             {object}  server.ErrorResponse  "if not authorized"
// @Failure      403             {object}  server.ErrorResponse  "if not allowed"
// @Failure      404             {object}  server.ErrorResponse  "if not found"
// @Failure      422             {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500             {object}  server.ErrorResponse
// @Failure      504             {object}  server.ErrorResponse  "if request times out"
// @Router       /users/{userId}/devices/{deviceUniqueId}/settings [GET].
func (s *service) GetDeviceSettings(ctx context.Context, r server.ParsedRequest) server.Response {
	resp, err := s.usersRepository.GetDeviceSettings(ctx, r.(*RequestGetDeviceSettings).DeviceID)
	if err != nil {
		err = errors.Wrapf(err, "failed to GetDeviceSettings for %#v", r.(*RequestGetDeviceSettings).DeviceID)
		switch {
		case errors.Is(err, users.ErrNotFound):
			return *server.NotFound(err, deviceSettingsNotFoundErrorCode)
		default:
			return server.Unexpected(err)
		}
	}

	return server.OK(resp)
}

func newRequestGetDeviceSettings() server.ParsedRequest {
	return new(RequestGetDeviceSettings)
}

func (req *RequestGetDeviceSettings) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestGetDeviceSettings) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestGetDeviceSettings) Validate() *server.Response {
	if req.AuthenticatedUser.ID != req.UserID {
		return server.Forbidden(errors.Errorf("you can only see the settings for your own devices. d>%#v!=a>%v", req.DeviceID, req.AuthenticatedUser.ID))
	}

	return server.RequiredStrings(map[string]string{"userId": req.UserID, "deviceUniqueId": req.DeviceUniqueID})
}

func (req *RequestGetDeviceSettings) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindUri, server.ShouldBindAuthenticatedUser(c)}
}
