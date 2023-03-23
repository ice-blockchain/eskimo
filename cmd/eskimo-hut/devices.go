// SPDX-License-Identifier: ice License 1.0

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
		PUT("users/:userId/devices/:deviceUniqueId/metadata/location", server.RootHandler(s.GetDeviceLocation))
}

// ReplaceDeviceMetadata godoc
//
//	@Schemes
//	@Description	Replaces existing device metadata with the provided one.
//	@Tags			Devices
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header	string								true	"Insert your access token"	default(Bearer <Add access token here>)
//	@Param			X-Forwarded-For	header	string								false	"Client IP"					default(1.1.1.1)
//	@Param			userId			path	string								true	"ID of the user"
//	@Param			deviceUniqueId	path	string								true	"ID of the device"
//	@Param			request			body	ReplaceDeviceMetadataRequestBody	true	"Request params"
//	@Success		200				"OK"
//	@Failure		400				{object}	server.ErrorResponse	"if validations fail"
//	@Failure		401				{object}	server.ErrorResponse	"if not authorized"
//	@Failure		403				{object}	server.ErrorResponse	"if not allowed"
//	@Failure		404				{object}	server.ErrorResponse	"if user not found"
//	@Failure		422				{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500				{object}	server.ErrorResponse
//	@Failure		504				{object}	server.ErrorResponse	"if request times out"
//	@Router			/users/{userId}/devices/{deviceUniqueId}/metadata [PUT].
func (s *service) ReplaceDeviceMetadata( //nolint:gocritic // False negative.
	ctx context.Context,
	req *server.Request[ReplaceDeviceMetadataRequestBody, any],
) (*server.Response[any], *server.Response[server.ErrorResponse]) {
	req.Data.DeviceMetadata.ID.DeviceUniqueID = req.Data.DeviceUniqueID
	req.Data.DeviceMetadata.ID.UserID = req.Data.UserID
	if req.AuthenticatedUser.UserID == "" && req.Data.DeviceMetadata.ID.UserID != "" && req.Data.DeviceMetadata.ID.UserID != "-" {
		return nil, server.Unauthorized(errors.New("authorization required"))
	}
	if err := s.usersProcessor.ReplaceDeviceMetadata(ctx, &req.Data.DeviceMetadata, req.ClientIP); err != nil {
		err = errors.Wrapf(err, "failed to ReplaceDeviceMetadata for %#v", req.Data)
		switch {
		case errors.Is(err, users.ErrRelationNotFound):
			return nil, server.NotFound(err, userNotFoundErrorCode)
		case errors.Is(err, users.ErrInvalidAppVersion):
			return nil, server.UnprocessableEntity(err, invalidPropertiesErrorCode)
		case errors.Is(err, users.ErrOutdatedAppVersion):
			return nil, server.BadRequest(err, deviceMetadataAppUpdateRequireErrorCode)
		default:
			return nil, server.Unexpected(err)
		}
	}

	return server.OK[any](), nil
}

// GetDeviceLocation godoc
//
//	@Schemes
//	@Description	Returns the device's geolocation based on its IP or based on account information if userId is also provided.
//	@Tags			Devices
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string	false	"Insert your access token. Required only if userId is set"	default(Bearer <Add access token here>)
//	@Param			X-Forwarded-For	header		string	false	"Client IP"													default(1.1.1.1)
//	@Param			userId			path		string	true	"ID of the user. Is optional, set an `-` if none."
//	@Param			deviceUniqueId	path		string	true	"ID of the device. Is optional, set an `-` if none."
//	@Success		200				{object}	users.DeviceLocation
//	@Failure		400				{object}	server.ErrorResponse	"if validations fail"
//	@Failure		401				{object}	server.ErrorResponse	"if not authenticated"
//	@Failure		403				{object}	server.ErrorResponse	"if not allowed"
//	@Failure		422				{object}	server.ErrorResponse	"if syntax fails"
//	@Failure		500				{object}	server.ErrorResponse
//	@Failure		504				{object}	server.ErrorResponse	"if request times out"
//	@Router			/users/{userId}/devices/{deviceUniqueId}/metadata/location [PUT].
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
	deviceID := &users.DeviceID{UserID: req.Data.UserID, DeviceUniqueID: req.Data.DeviceUniqueID}

	return server.OK(s.usersProcessor.GetDeviceMetadataLocation(ctx, deviceID, req.ClientIP)), nil
}
