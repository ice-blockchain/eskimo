// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	repeatNum = 20
)

type (
	response struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}

	getUserDevicesArg struct {
		reqUserID          string
		reqDeviceID        string
		expectedRespErr    string
		expectedRespCode   string
		reqHeaders         []http.Header
		expectedRespStatus int
	}
)

func TestEskimo_GetUserDevices_NotFound(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)

	repeatWithParallel(t, "GetUserDevices not found -> 404.", func() {
		deviceUniqueID := uuid.NewString()
		doGetUserDevices(ctx, t, &getUserDevicesArg{
			reqUserID:   testUserID,
			reqDeviceID: deviceUniqueID,
			//nolint:lll // Here is the string to compare with.
			expectedRespErr:    fmt.Sprintf(`failed to GetDeviceSettings for device.ID{_msgpack:struct {}{}, UserID:"%v", DeviceUniqueID:"%v"}: not found`, testUserID, deviceUniqueID),
			expectedRespCode:   "DEVICE_SETTINGS_NOT_FOUND",
			expectedRespStatus: http.StatusNotFound,
			reqHeaders:         []http.Header{authHeader()},
		})
	})
}

func TestEskimo_GetUserDevices_Unauthorized(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)

	repeatWithParallel(t, "GetUserDevices unauthorized -> 401.", func() {
		deviceUniqueID := uuid.NewString()
		userID := generateUserID(t)
		doGetUserDevices(ctx, t, &getUserDevicesArg{
			reqUserID:          userID,
			reqDeviceID:        deviceUniqueID,
			expectedRespErr:    `unexpected end of JSON input`,
			expectedRespCode:   "INVALID_TOKEN",
			expectedRespStatus: http.StatusUnauthorized,
		})
	})
}

func TestEskimo_GetUserDevices_OperationNotAllowed(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)

	repeatWithParallel(t, "GetUserDevices operation not allowed -> 403.", func() {
		deviceUniqueID := uuid.NewString()
		userID := generateUserID(t)
		doGetUserDevices(ctx, t, &getUserDevicesArg{
			reqUserID:   userID,
			reqDeviceID: deviceUniqueID,
			//nolint:lll // Here is the string to compare with.
			expectedRespErr:    fmt.Sprintf(`you can only see the settings for your own devices. d>device.ID{_msgpack:struct {}{}, UserID:"%v", DeviceUniqueID:"%v"}!=a>%v`, userID, deviceUniqueID, testUserID),
			expectedRespCode:   "OPERATION_NOT_ALLOWED",
			expectedRespStatus: http.StatusForbidden,
			reqHeaders:         []http.Header{authHeader()},
		})
	})
}

func doGetUserDevices(ctx context.Context, tb testing.TB, arg *getUserDevicesArg) {
	tb.Helper()

	body, status, headers := serverConnector.Get(ctx, tb, fmt.Sprintf(`/v1r/users/%v/devices/%v/settings`, arg.reqUserID, arg.reqDeviceID), arg.reqHeaders...)
	resp := new(response)
	require.NoError(tb, json.Unmarshal([]byte(body), resp))
	assert.Equal(tb, resp.Code, arg.expectedRespCode)
	assert.Equal(tb, resp.Error, arg.expectedRespErr)
	assert.Equal(tb, arg.expectedRespStatus, status)
	l, err := strconv.Atoi(headers.Get("Content-Length"))
	require.NoError(tb, err)
	assert.Greater(tb, l, 0)
	headers.Del("Date")
	headers.Del("Content-Length")
	assert.Equal(tb, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, headers)
}

func authHeader() http.Header {
	return http.Header{"Authorization": []string{testDIDToken}}
}

func randomHex(t *testing.T, num int) string {
	t.Helper()
	//nolint:makezero // Because otherwise we have empty value.
	bytes := make([]byte, num)
	n, err := rand.Read(bytes)
	assert.Equal(t, n, len(bytes))
	require.NoError(t, err)

	return hex.EncodeToString(bytes)
}

func generateUserID(t *testing.T) string {
	t.Helper()

	return "did:ethr:0x" + randomHex(t, 20)
}

func repeatWithParallel(t *testing.T, testName string, f func()) {
	t.Helper()
	for i := 0; i < repeatNum; i++ {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			f()
		})
	}
}
