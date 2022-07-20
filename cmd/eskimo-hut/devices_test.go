// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
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
	responseMeta struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}

	responseData struct {
		Headers http.Header
		Body    string
		Status  int
	}

	testUserDevicesRequestDataArg struct {
		reqUserID             string
		reqDeviceID           string
		reqBody               string
		expectedRespRegexpErr string
		expectedRespCode      string
		reqHeaders            []http.Header
		expectedRespStatus    int
	}
)

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestEskimoHut_CreateDeviceSettings_Success(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)
	repeatWithParallel(t, "CreateDeviceSettings success -> 201.", func() {
		deviceUniqueID := uuid.NewString()
		doCreateUserDevices(ctx, t, &testUserDevicesRequestDataArg{
			reqUserID:             testUserID,
			reqDeviceID:           deviceUniqueID,
			reqBody:               generateReqBody(t, testUserID, deviceUniqueID),
			expectedRespRegexpErr: ``,
			expectedRespCode:      ``,
			reqHeaders:            []http.Header{authHeader()},
			expectedRespStatus:    http.StatusCreated,
		})
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestEskimoHut_CreateDeviceSettings_Failure_Duplicate(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)
	repeatWithParallel(t, "CreateDeviceSettings failure duplicate -> 409.", func() {
		deviceUniqueID := uuid.NewString()
		reqBody := generateReqBody(t, testUserID, deviceUniqueID)
		doCreateUserDevices(ctx, t, &testUserDevicesRequestDataArg{
			reqUserID:             testUserID,
			reqDeviceID:           deviceUniqueID,
			reqBody:               reqBody,
			expectedRespRegexpErr: ``,
			expectedRespCode:      ``,
			reqHeaders:            []http.Header{authHeader()},
			expectedRespStatus:    http.StatusCreated,
		})
		//nolint:lll // Here is the string to regexp with.
		errMsg := fmt.Sprintf(`failed to CreateDeviceSettings for &devicesettings\.DeviceSettings.+ID:device\.ID{_msgpack:struct {}{}, UserID:"%[1]v", DeviceUniqueID:"%[2]v"}}: failed to insert &devicesettings\.DeviceSettings.+ID:device\.ID{_msgpack:struct {}{}, UserID:"%[1]v", DeviceUniqueID:"%[2]v"}}: failed to insert.+ID:device\.ID{_msgpack:struct {}{}, UserID:"%[1]v", DeviceUniqueID:"%[2]v"}}: duplicate`, testUserID, deviceUniqueID)
		doCreateUserDevices(ctx, t, &testUserDevicesRequestDataArg{
			reqUserID:             testUserID,
			reqDeviceID:           deviceUniqueID,
			reqBody:               reqBody,
			expectedRespRegexpErr: errMsg,
			expectedRespCode:      `DEVICE_SETTINGS_ALREADY_EXISTS`,
			reqHeaders:            []http.Header{authHeader()},
			expectedRespStatus:    http.StatusConflict,
		})
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestEskimoHut_CreateDeviceSettings_Failure_SyntaxFails(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)
	repeatWithParallel(t, "CreateDeviceSettings syntax fails failure -> 422.", func() {
		doCreateUserDevices(ctx, t, &testUserDevicesRequestDataArg{
			reqUserID:             testUserID,
			reqDeviceID:           uuid.NewString(),
			reqBody:               ``,
			expectedRespRegexpErr: `EOF`,
			expectedRespCode:      `STRUCTURE_VALIDATION_FAILED`,
			reqHeaders:            []http.Header{authHeader()},
			expectedRespStatus:    http.StatusUnprocessableEntity,
		})
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestEskimoHut_CreateDeviceSettings_Failure_Forbidden(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)
	repeatWithParallel(t, "CreateDeviceSettings failure forbidden -> 403.", func() {
		userID := generateUserID(t)
		deviceUniqueID := uuid.NewString()
		reqBody := generateReqBody(t, userID, deviceUniqueID)
		//nolint:lll // Here is the long string to regexp with.
		regexpErrMsg := fmt.Sprintf(`you can only create the settings for your own devices. d>device.ID{_msgpack:struct {}{}, UserID:"%v", DeviceUniqueID:"%v"}!=a>%v`, userID, deviceUniqueID, testUserID)
		doCreateUserDevices(ctx, t, &testUserDevicesRequestDataArg{
			reqUserID:             testUserID,
			reqDeviceID:           uuid.NewString(),
			reqBody:               reqBody,
			expectedRespRegexpErr: regexpErrMsg,
			expectedRespCode:      `OPERATION_NOT_ALLOWED`,
			reqHeaders:            []http.Header{authHeader()},
			expectedRespStatus:    http.StatusForbidden,
		})
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestEskimoHut_CreateDeviceSettings_Failure_InvalidProperties(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)
	repeatWithParallel(t, "CreateDeviceSettings failure invalid properties -> 400.", func() {
		deviceUniqueID := uuid.NewString()
		doCreateUserDevices(ctx, t, &testUserDevicesRequestDataArg{
			reqUserID:             testUserID,
			reqDeviceID:           deviceUniqueID,
			reqBody:               fmt.Sprintf(`{"userId":"%v","deviceUniqueId":"%v"}`, testUserID, deviceUniqueID),
			expectedRespRegexpErr: `no properties provided for update`,
			expectedRespCode:      `INVALID_PROPERTIES`,
			reqHeaders:            []http.Header{authHeader()},
			expectedRespStatus:    http.StatusBadRequest,
		})
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestEskimoHut_CreateDeviceSettings_Failure_Unauthorized(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)
	repeatWithParallel(t, "CreateDeviceSettings failure unauthorized -> 401.", func() {
		deviceUniqueID := uuid.NewString()
		reqBody := generateReqBody(t, generateUserID(t), deviceUniqueID)
		doCreateUserDevices(ctx, t, &testUserDevicesRequestDataArg{
			reqUserID:             testUserID,
			reqDeviceID:           deviceUniqueID,
			reqBody:               reqBody,
			expectedRespRegexpErr: `unexpected end of JSON input`,
			expectedRespCode:      `INVALID_TOKEN`,
			expectedRespStatus:    http.StatusUnauthorized,
		})
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestEskimoHut_ModifyDeviceSettings_Success(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)
	repeatWithParallel(t, "ModifyDeviceSettings success -> 200.", func() {
		deviceUniqueID := uuid.NewString()
		reqBody := generateReqBody(t, testUserID, deviceUniqueID)
		doCreateUserDevices(ctx, t, &testUserDevicesRequestDataArg{
			reqUserID:             testUserID,
			reqDeviceID:           deviceUniqueID,
			reqBody:               reqBody,
			expectedRespRegexpErr: ``,
			expectedRespCode:      ``,
			reqHeaders:            []http.Header{authHeader()},
			expectedRespStatus:    http.StatusCreated,
		})

		reqModifiedBody := generateReqBody(t, testUserID, deviceUniqueID)
		doModifyUserDevices(ctx, t, &testUserDevicesRequestDataArg{
			reqUserID:             testUserID,
			reqDeviceID:           deviceUniqueID,
			reqBody:               reqModifiedBody,
			expectedRespRegexpErr: ``,
			expectedRespCode:      ``,
			reqHeaders:            []http.Header{authHeader()},
			expectedRespStatus:    http.StatusOK,
		})
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestEskimoHut_ModifyDeviceSettings_Failure_NotFound(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)
	repeatWithParallel(t, "ModifyDeviceSettings failure not found -> 404.", func() {
		deviceUniqueID := uuid.NewString()
		reqBody := generateReqBody(t, testUserID, deviceUniqueID)
		//nolint:lll // Here is the long string to regexp with.
		msgRegexpErr := fmt.Sprintf(`failed to ModifyDeviceSettings for &devicesettings\.DeviceSettings.+ID:device\.ID{_msgpack:struct {}{}, UserID:"%[1]v", DeviceUniqueID:"%[2]v"}}: failed to get current device settings for device\.ID{_msgpack:struct {}{}, UserID:"%[1]v", DeviceUniqueID:"%[2]v"}: not found`, testUserID, deviceUniqueID)
		doModifyUserDevices(ctx, t, &testUserDevicesRequestDataArg{
			reqUserID:             testUserID,
			reqDeviceID:           deviceUniqueID,
			reqBody:               reqBody,
			expectedRespRegexpErr: msgRegexpErr,
			expectedRespCode:      `DEVICE_SETTINGS_NOT_FOUND`,
			reqHeaders:            []http.Header{authHeader()},
			expectedRespStatus:    http.StatusNotFound,
		})
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestEskimoHut_ModifyDeviceSettings_Failure_Unauthorized(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)
	repeatWithParallel(t, "ModifyDeviceSettings failure unauthorized -> 401.", func() {
		deviceUniqueID := uuid.NewString()
		userID := generateUserID(t)
		reqBody := generateReqBody(t, userID, deviceUniqueID)
		doModifyUserDevices(ctx, t, &testUserDevicesRequestDataArg{
			reqUserID:             testUserID,
			reqDeviceID:           deviceUniqueID,
			reqBody:               reqBody,
			expectedRespRegexpErr: `unexpected end of JSON input`,
			expectedRespCode:      `INVALID_TOKEN`,
			expectedRespStatus:    http.StatusUnauthorized,
		})
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestEskimoHut_ModifyDeviceSettings_Failure_Forbidden(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)
	repeatWithParallel(t, "ModifyDeviceSettings failure forbidden -> 403.", func() {
		userID := generateUserID(t)
		deviceUniqueID := uuid.NewString()
		reqBody := generateReqBody(t, userID, deviceUniqueID)
		doModifyUserDevices(ctx, t, &testUserDevicesRequestDataArg{
			reqUserID:   testUserID,
			reqDeviceID: deviceUniqueID,
			reqBody:     reqBody,
			//nolint:lll // Here is the long string to compare with.
			expectedRespRegexpErr: fmt.Sprintf(`you can only modify the settings for your own devices. d>device.ID{_msgpack:struct {}{}, UserID:"%v", DeviceUniqueID:"%v"}!=a>%v`, userID, deviceUniqueID, testUserID),
			expectedRespCode:      `OPERATION_NOT_ALLOWED`,
			reqHeaders:            []http.Header{authHeader()},
			expectedRespStatus:    http.StatusForbidden,
		})
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestEskimoHut_ModifyDeviceSettings_Failure_InvalidProperties(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)
	repeatWithParallel(t, "ModifyDeviceSettings failure invalid properties -> 400.", func() {
		deviceUniqueID := uuid.NewString()
		reqBody := fmt.Sprintf(`{"userId":"%v","deviceUniqueId":"%v"}`, testUserID, deviceUniqueID)
		doModifyUserDevices(ctx, t, &testUserDevicesRequestDataArg{
			reqUserID:             testUserID,
			reqDeviceID:           deviceUniqueID,
			reqBody:               reqBody,
			expectedRespRegexpErr: `no properties provided for update`,
			expectedRespCode:      `INVALID_PROPERTIES`,
			reqHeaders:            []http.Header{authHeader()},
			expectedRespStatus:    http.StatusBadRequest,
		})
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestEskimoHut_ModifyDeviceSettings_Failure_SyntaxFails(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)
	repeatWithParallel(t, "ModifyDeviceSettings failure syntax fails -> 422.", func() {
		doCreateUserDevices(ctx, t, &testUserDevicesRequestDataArg{
			reqUserID:             testUserID,
			reqDeviceID:           uuid.NewString(),
			reqBody:               ``,
			expectedRespRegexpErr: `EOF`,
			expectedRespCode:      `STRUCTURE_VALIDATION_FAILED`,
			reqHeaders:            []http.Header{authHeader()},
			expectedRespStatus:    http.StatusUnprocessableEntity,
		})
	})
}

func authHeader() http.Header {
	return http.Header{"Authorization": []string{testDIDToken}}
}

func doCreateUserDevices(ctx context.Context, tb testing.TB, arg *testUserDevicesRequestDataArg) {
	tb.Helper()
	reqReader, _ := serverConnector.WrapJSONBody(arg.reqBody)
	url := fmt.Sprintf(`/v1w/users/%v/devices/%v/settings`, arg.reqUserID, arg.reqDeviceID)
	body, status, headers := serverConnector.Post(ctx, tb, url, reqReader, arg.reqHeaders...)
	doHandleUserDevices(tb, arg, &responseData{Body: body, Status: status, Headers: headers})
}

func doModifyUserDevices(ctx context.Context, tb testing.TB, arg *testUserDevicesRequestDataArg) {
	tb.Helper()
	reqReader, _ := serverConnector.WrapJSONBody(arg.reqBody)
	url := fmt.Sprintf(`/v1w/users/%v/devices/%v/settings`, arg.reqUserID, arg.reqDeviceID)
	body, status, headers := serverConnector.Patch(ctx, tb, url, reqReader, arg.reqHeaders...)
	doHandleUserDevices(tb, arg, &responseData{Body: body, Status: status, Headers: headers})
}

func doHandleUserDevices(tb testing.TB, arg *testUserDevicesRequestDataArg, respData *responseData) {
	tb.Helper()
	resp := new(responseMeta)
	require.NoError(tb, json.Unmarshal([]byte(respData.Body), resp))
	assert.Equal(tb, resp.Code, arg.expectedRespCode)
	if resp.Error == "" {
		assert.Equal(tb, resp.Error, arg.expectedRespRegexpErr)
	} else {
		assert.Regexp(tb, regexp.MustCompile(arg.expectedRespRegexpErr), resp.Error)
	}
	assert.Equal(tb, arg.expectedRespStatus, respData.Status)
	l, err := strconv.Atoi(respData.Headers.Get("Content-Length"))
	require.NoError(tb, err)
	assert.Greater(tb, l, 0)
	respData.Headers.Del("Date")
	respData.Headers.Del("Content-Length")
	assert.Equal(tb, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, respData.Headers)
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

func repeatWithParallel(t *testing.T, funcName string, f func()) {
	t.Helper()
	for i := 0; i < repeatNum; i++ {
		t.Run(funcName, func(t *testing.T) {
			t.Parallel()
			f()
		})
	}
}

//nolint:lll // This is the long notification settings JSON data with random true/false values.
func generateReqBody(t *testing.T, userID, uniqueDeviceID string) string {
	t.Helper()
	channels := fmt.Sprintf(`{"push":%v,"email":%v,"sms":%v,"inApp":%v}`, *randBool(t), *randBool(t), *randBool(t), *randBool(t))

	return fmt.Sprintf(`{"notificationSettings":{"ACHIEVEMENTS":%[1]v,"NEWS":%[1]v,"REMINDERS":%[1]v,"TEAM":%[1]v},"language":"86","disableAllNotifications":false,"userId":"%[2]v","deviceUniqueId":"%[3]v"}`, channels, userID, uniqueDeviceID)
}

func randBool(t *testing.T) *bool {
	t.Helper()
	num, err := rand.Int(rand.Reader, big.NewInt(2))
	require.NoError(t, err)
	var val bool
	if num.Int64() == 1 {
		val = true
	} else {
		val = false
	}

	return &val
}
