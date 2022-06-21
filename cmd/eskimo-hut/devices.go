// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/server"
)

func (s *service) setupDevicesRoutes(router *gin.Engine) {
	router.
		Group("v1").
		PUT("users/:userId/devices/:deviceUniqueId/metadata", server.RootHandler(newRequestReplaceDeviceMetadata, s.ReplaceDeviceMetadata)).
		PATCH("users/:userId/devices/:deviceUniqueId/settings", server.RootHandler(newRequestModifyDeviceSettings, s.ModifyDeviceSettings)).
		PUT("users/:userId/devices/:deviceUniqueId/metadata/location", server.RootHandler(newRequestGetDeviceLocation, s.GetDeviceLocation))
}

// ReplaceDeviceMetadata godoc
// @Schemes
// @Description  Replaces existing device metadata with the provided one.
// @Tags         Devices
// @Accept       json
// @Produce      json
// @Param        Authorization   header  string                true  "Insert your access token"  default(Bearer <Add access token here>)
// @Param        userId          path    string                true  "ID of the user"
// @Param        deviceUniqueId  path    string                true  "ID of the device"
// @Param        request         body    users.DeviceMetadata  true  "Request params"
// @Success      200             "OK"
// @Failure      400             {object}  server.ErrorResponse  "if validations fail"
// @Failure      401             {object}  server.ErrorResponse  "if not authorized"
// @Failure      403             {object}  server.ErrorResponse  "if not allowed"
// @Failure      422             {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500             {object}  server.ErrorResponse
// @Failure      504             {object}  server.ErrorResponse  "if request times out"
// @Router       /users/{userId}/devices/{deviceUniqueId}/metadata [PUT].
func (s *service) ReplaceDeviceMetadata(ctx context.Context, r server.ParsedRequest) server.Response {
	if err := s.usersProcessor.ReplaceDeviceMetadata(ctx, &r.(*RequestReplaceDeviceMetadata).ReplaceDeviceMetadataArg); err != nil {
		return server.Unexpected(errors.Wrapf(err, "failed to ReplaceDeviceMetadata for %#v", &r.(*RequestReplaceDeviceMetadata).ReplaceDeviceMetadataArg))
	}

	return server.OK()
}

func newRequestReplaceDeviceMetadata() server.ParsedRequest {
	return new(RequestReplaceDeviceMetadata)
}

func (req *RequestReplaceDeviceMetadata) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestReplaceDeviceMetadata) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestReplaceDeviceMetadata) SetClientIP(ip net.IP) {
	if len(req.ClientIP) == 0 {
		req.ClientIP = ip
	}
}

func (req *RequestReplaceDeviceMetadata) GetClientIP() net.IP {
	return req.ClientIP
}

func (req *RequestReplaceDeviceMetadata) Validate() *server.Response {
	if req.AuthenticatedUser.ID != req.UserID {
		return server.Forbidden(errors.Errorf("you can only replace the metadata for your own devices. d>%#v!=a>%v", req.DeviceID, req.AuthenticatedUser.ID))
	}

	return server.RequiredStrings(map[string]string{"userId": req.UserID, "deviceUniqueId": req.ID.DeviceUniqueID})
}

func (req *RequestReplaceDeviceMetadata) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindUri, c.ShouldBindJSON, server.ShouldBindClientIP(c), server.ShouldBindAuthenticatedUser(c)}
}

// ModifyDeviceSettings godoc
// @Schemes
// @Description  Modifies only specific device settings provided in the request body.
// @Tags         Devices
// @Accept       json
// @Produce      json
// @Param        Authorization   header  string                true  "Insert your access token"  default(Bearer <Add access token here>)
// @Param        userId          path    string                true  "ID of the user"
// @Param        deviceUniqueId  path    string                true  "ID of the device"
// @Param        request         body    users.DeviceSettings  true  "Request params"
// @Success      200             "OK"
// @Failure      400             {object}  server.ErrorResponse  "if validations fail"
// @Failure      401             {object}  server.ErrorResponse  "if not authorized"
// @Failure      403             {object}  server.ErrorResponse  "if not allowed"
// @Failure      422             {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500             {object}  server.ErrorResponse
// @Failure      504             {object}  server.ErrorResponse  "if request times out"
// @Router       /users/{userId}/devices/{deviceUniqueId}/settings [PATCH].
func (s *service) ModifyDeviceSettings(ctx context.Context, r server.ParsedRequest) server.Response {
	if err := s.usersProcessor.ModifyDeviceSettings(ctx, &r.(*RequestModifyDeviceSettings).DeviceSettings); err != nil {
		return server.Unexpected(errors.Wrapf(err, "failed to ModifyDeviceSettings for %#v", &r.(*RequestModifyDeviceSettings).DeviceSettings))
	}

	return server.OK()
}

func newRequestModifyDeviceSettings() server.ParsedRequest {
	return new(RequestModifyDeviceSettings)
}

func (req *RequestModifyDeviceSettings) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestModifyDeviceSettings) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestModifyDeviceSettings) Validate() *server.Response {
	if req.AuthenticatedUser.ID != req.UserID {
		return server.Forbidden(errors.Errorf("you can only modify the settings for your own devices. d>%#v!=a>%v", req.ID, req.AuthenticatedUser.ID))
	}
	if len(req.NotificationSettings) == 0 && req.Language == "" {
		return server.BadRequest(errors.New("no properties provided for update"), invalidPropertiesErrorCode)
	}

	return server.RequiredStrings(map[string]string{"userId": req.UserID, "deviceUniqueId": req.ID.DeviceUniqueID})
}

func (req *RequestModifyDeviceSettings) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindUri, c.ShouldBindJSON, server.ShouldBindAuthenticatedUser(c)}
}

// GetDeviceLocation godoc
// @Schemes
// @Description  Returns the device's geolocation based on its IP or based on account information if userId is also provided.
// @Tags         Devices
// @Accept       json
// @Produce      json
// @Param        Authorization   header    string  false  "Insert your access token. Required only if userId is set"  default(Bearer <Add access token here>)
// @Param        userId          path      string  true   "ID of the user. Is optional, set an `-` if none."
// @Param        deviceUniqueId  path      string  true   "ID of the device"
// @Success      200             {object}  users.DeviceLocation
// @Failure      400             {object}  server.ErrorResponse  "if validations fail"
// @Failure      401             {object}  server.ErrorResponse  "if not authenticated"
// @Failure      403             {object}  server.ErrorResponse  "if not allowed"
// @Failure      422             {object}  server.ErrorResponse  "if syntax fails"
// @Failure      500             {object}  server.ErrorResponse
// @Failure      504             {object}  server.ErrorResponse  "if request times out"
// @Router       /users/{userId}/devices/{deviceUniqueId}/metadata/location [PUT].
func (s *service) GetDeviceLocation(ctx context.Context, r server.ParsedRequest) server.Response {
	return server.OK(s.usersProcessor.GetDeviceMetadataLocation(ctx, &r.(*RequestGetDeviceLocation).GetDeviceMetadataLocationArg))
}

func newRequestGetDeviceLocation() server.ParsedRequest {
	return new(RequestGetDeviceLocation)
}

func (req *RequestGetDeviceLocation) SetAuthenticatedUser(user server.AuthenticatedUser) {
	if req.AuthenticatedUser.ID == "" {
		req.AuthenticatedUser = user
	}
}

func (req *RequestGetDeviceLocation) GetAuthenticatedUser() server.AuthenticatedUser {
	return req.AuthenticatedUser
}

func (req *RequestGetDeviceLocation) ShouldAuthenticateUser(ginCtx *gin.Context) bool {
	userID := strings.Trim(ginCtx.Param("userId"), " ")

	return userID != "" && userID != "-"
}

func (req *RequestGetDeviceLocation) SetClientIP(ip net.IP) {
	if len(req.ClientIP) == 0 {
		req.ClientIP = ip
	}
}

func (req *RequestGetDeviceLocation) GetClientIP() net.IP {
	return req.ClientIP
}

func (req *RequestGetDeviceLocation) Validate() *server.Response {
	if req.UserID == "-" {
		req.UserID = ""
	}
	if req.AuthenticatedUser.ID != req.UserID {
		return server.Forbidden(errors.Errorf("you can get device location only for your devices. u>%v!=a>%v", req.UserID, req.AuthenticatedUser.ID))
	}

	return server.RequiredStrings(map[string]string{"deviceUniqueId": req.ID.DeviceUniqueID})
}

func (req *RequestGetDeviceLocation) Bindings(c *gin.Context) []func(obj interface{}) error {
	return []func(obj interface{}) error{c.ShouldBindUri, server.ShouldBindAuthenticatedUser(c), server.ShouldBindClientIP(c)}
}
