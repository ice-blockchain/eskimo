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
		Group("v1w").
		PUT("users/:userId/devices/:deviceUniqueId/metadata", server.RootHandler(s.ReplaceDeviceMetadata)).
		PATCH("users/:userId/devices/:deviceUniqueId/settings", server.RootHandler(s.ModifyDeviceSettings)).
		POST("users/:userId/devices/:deviceUniqueId/settings", server.RootHandler(s.CreateDeviceSettings)).
		PUT("users/:userId/devices/:deviceUniqueId/metadata/location", server.RootHandler(s.GetDeviceLocation))
}

// ReplaceDeviceMetadata godoc
// @Schemes
// @Description Replaces existing device metadata with the provided one.
// @Tags        Devices
// @Accept      json
// @Produce     json
// @Param       Authorization  header string                   true "Insert your access token" default(Bearer <Add access token here>)
// @Param       userId         path   string                   true "ID of the user"
// @Param       deviceUniqueId path   string                   true "ID of the device"
// @Param       request        body   ReplaceDeviceMetadataArg true "Request params"
// @Success     200            "OK"
// @Failure     400            {object} server.ErrorResponse "if validations fail"
// @Failure     401            {object} server.ErrorResponse "if not authorized"
// @Failure     403            {object} server.ErrorResponse "if not allowed"
// @Failure     422            {object} server.ErrorResponse "if syntax fails"
// @Failure     500            {object} server.ErrorResponse
// @Failure     504            {object} server.ErrorResponse "if request times out"
// @Router      /users/{userId}/devices/{deviceUniqueId}/metadata [PUT].
func (s *service) ReplaceDeviceMetadata( //nolint:gocritic // False negative.
	ctx context.Context,
	req *server.Request[ReplaceDeviceMetadataArg, any],
) (*server.Response[any], *server.Response[server.ErrorResponse]) {
	req.Data.DeviceMetadata.ID.DeviceUniqueID = req.Data.DeviceUniqueID
	req.Data.DeviceMetadata.ID.UserID = req.Data.UserID
	if err := s.usersProcessor.ReplaceDeviceMetadata(ctx, &req.Data.DeviceMetadata, req.ClientIP); err != nil {
		return nil, server.Unexpected(errors.Wrapf(err, "failed to ReplaceDeviceMetadata for %#v", req.Data))
	}

	return server.OK[any](), nil
}

// ModifyDeviceSettings godoc
// @Schemes
// @Description Modifies only specific device settings provided in the request body.
// @Tags        Devices
// @Accept      json
// @Produce     json
// @Param       Authorization  header   string                  true "Insert your access token" default(Bearer <Add access token here>)
// @Param       userId         path     string                  true "ID of the user"
// @Param       deviceUniqueId path     string                  true "ID of the device"
// @Param       request        body     ModifyDeviceSettingsArg true "Request params"
// @Success     200            {object} users.DeviceSettings    "updated result"
// @Failure     400            {object} server.ErrorResponse    "if validations fail"
// @Failure     401            {object} server.ErrorResponse    "if not authorized"
// @Failure     403            {object} server.ErrorResponse    "if not allowed"
// @Failure     404            {object} server.ErrorResponse    "if not found"
// @Failure     422            {object} server.ErrorResponse    "if syntax fails"
// @Failure     500            {object} server.ErrorResponse
// @Failure     504            {object} server.ErrorResponse "if request times out"
// @Router      /users/{userId}/devices/{deviceUniqueId}/settings [PATCH].
func (s *service) ModifyDeviceSettings( //nolint:dupl,gocritic // That's intended.
	ctx context.Context,
	req *server.Request[ModifyDeviceSettingsArg, users.DeviceSettings],
) (*server.Response[users.DeviceSettings], *server.Response[server.ErrorResponse]) {
	if req.Data.NotificationSettings == nil &&
		req.Data.Language == nil &&
		req.Data.DisableAllNotifications == nil {
		return nil, server.UnprocessableEntity(errors.New("no properties provided for update"), invalidPropertiesErrorCode)
	}
	ds := &users.DeviceSettings{
		NotificationSettings:    req.Data.NotificationSettings,
		Language:                req.Data.Language,
		DisableAllNotifications: req.Data.DisableAllNotifications,
		ID: users.DeviceID{
			UserID:         req.Data.UserID,
			DeviceUniqueID: req.Data.DeviceUniqueID,
		},
	}
	if err := s.usersProcessor.ModifyDeviceSettings(ctx, ds); err != nil {
		err = errors.Wrapf(err, "failed to ModifyDeviceSettings for %#v", req.Data)
		switch {
		case errors.Is(err, users.ErrNotFound):
			return nil, server.NotFound(err, deviceSettingsNotFoundErrorCode)
		default:
			return nil, server.Unexpected(err)
		}
	}

	return server.OK(ds), nil
}

// CreateDeviceSettings godoc
// @Schemes
// @Description Creates initial device settings provided in the request body.
// @Tags        Devices
// @Accept      json
// @Produce     json
// @Param       Authorization  header   string                  true "Insert your access token" default(Bearer <Add access token here>)
// @Param       userId         path     string                  true "ID of the user"
// @Param       deviceUniqueId path     string                  true "ID of the device"
// @Param       request        body     CreateDeviceSettingsArg true "Request params"
// @Success     201            {object} users.DeviceSettings    "created result"
// @Failure     400            {object} server.ErrorResponse    "if validations fail"
// @Failure     401            {object} server.ErrorResponse    "if not authorized"
// @Failure     403            {object} server.ErrorResponse    "if not allowed"
// @Failure     409            {object} server.ErrorResponse    "if already exists"
// @Failure     422            {object} server.ErrorResponse    "if syntax fails"
// @Failure     500            {object} server.ErrorResponse
// @Failure     504            {object} server.ErrorResponse "if request times out"
// @Router      /users/{userId}/devices/{deviceUniqueId}/settings [POST].
func (s *service) CreateDeviceSettings( //nolint:dupl,gocritic // That's intended.
	ctx context.Context,
	req *server.Request[CreateDeviceSettingsArg, users.DeviceSettings],
) (*server.Response[users.DeviceSettings], *server.Response[server.ErrorResponse]) {
	if req.Data.NotificationSettings == nil &&
		req.Data.Language == nil &&
		req.Data.DisableAllNotifications == nil {
		return nil, server.UnprocessableEntity(errors.New("no properties provided for update"), invalidPropertiesErrorCode)
	}
	ds := &users.DeviceSettings{
		NotificationSettings:    req.Data.NotificationSettings,
		Language:                req.Data.Language,
		DisableAllNotifications: req.Data.DisableAllNotifications,
		ID: users.DeviceID{
			UserID:         req.Data.UserID,
			DeviceUniqueID: req.Data.DeviceUniqueID,
		},
	}
	if err := s.usersProcessor.CreateDeviceSettings(ctx, ds); err != nil {
		err = errors.Wrapf(err, "failed to CreateDeviceSettings for %#v", req.Data)
		switch {
		case errors.Is(err, users.ErrDuplicate):
			return nil, server.Conflict(err, deviceSettingsAlreadyExistsErrorCode)
		default:
			return nil, server.Unexpected(err)
		}
	}

	return server.Created(ds), nil
}

// GetDeviceLocation godoc
// @Schemes
// @Description Returns the device's geolocation based on its IP or based on account information if userId is also provided.
// @Tags        Devices
// @Accept      json
// @Produce     json
// @Param       Authorization  header   string false "Insert your access token. Required only if userId is set" default(Bearer <Add access token here>)
// @Param       userId         path     string true  "ID of the user. Is optional, set an `-` if none."
// @Param       deviceUniqueId path     string true  "ID of the device. Is optional, set an `-` if none."
// @Success     200            {object} users.DeviceLocation
// @Failure     400            {object} server.ErrorResponse "if validations fail"
// @Failure     401            {object} server.ErrorResponse "if not authenticated"
// @Failure     403            {object} server.ErrorResponse "if not allowed"
// @Failure     422            {object} server.ErrorResponse "if syntax fails"
// @Failure     500            {object} server.ErrorResponse
// @Failure     504            {object} server.ErrorResponse "if request times out"
// @Router      /users/{userId}/devices/{deviceUniqueId}/metadata/location [PUT].
func (s *service) GetDeviceLocation( //nolint:gocritic // False negative.
	ctx context.Context,
	req *server.Request[GetDeviceLocationArg, users.DeviceLocation],
) (*server.Response[users.DeviceLocation], *server.Response[server.ErrorResponse]) {
	if req.Data.UserID == "-" {
		req.Data.UserID = ""
	}
	if req.Data.DeviceUniqueID == "-" {
		req.Data.DeviceUniqueID = ""
	}
	deviceID := users.DeviceID{UserID: req.Data.UserID, DeviceUniqueID: req.Data.DeviceUniqueID}

	return server.OK(s.usersProcessor.GetDeviceMetadataLocation(ctx, deviceID, req.ClientIP)), nil
}
