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
	repeatNum = 3
)

type (
	response struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}

	request struct {
		reqUserID   string
		reqDeviceID string
		reqHeaders  []http.Header
	}
)

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestEskimo_GetUserDevices_Failure_NotFound(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)

	repeatScenarioInParallel(t, func() {
		givenDeviceUniqueID := uuid.NewString()
		givenUserID := testUserID

		//nolint:lll // Here is the string to compare with.
		expectedRespErr := fmt.Sprintf(`failed to GetDeviceSettings for &main.GetDeviceSettingsArg{UserID:"%v", DeviceUniqueID:"%v"}: not found`, givenUserID, givenDeviceUniqueID)
		expectedRespCode := "DEVICE_SETTINGS_NOT_FOUND"

		arg := request{reqUserID: givenUserID, reqDeviceID: givenDeviceUniqueID, reqHeaders: []http.Header{authHeader()}}
		resp, status, headers := whenDoGetUserDevices(ctx, t, &arg)

		resp.thenExpectResponse(t, expectedRespCode, expectedRespErr)
		thenExpectStatus(t, status, http.StatusNotFound)
		thenExpectHeaders(t, headers)
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestEskimo_GetUserDevices_Failure_Unauthorized(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)

	repeatScenarioInParallel(t, func() {
		givenDeviceUniqueID := uuid.NewString()
		givenUserID := shouldGenerateRandomUserID(t)

		expectedRespErr := `unexpected end of JSON input`
		expectedRespCode := "INVALID_TOKEN"

		arg := request{reqUserID: givenUserID, reqDeviceID: givenDeviceUniqueID}
		resp, status, headers := whenDoGetUserDevices(ctx, t, &arg)

		resp.thenExpectResponse(t, expectedRespCode, expectedRespErr)
		thenExpectStatus(t, status, http.StatusUnauthorized)
		thenExpectHeaders(t, headers)
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestEskimo_GetUserDevices_Failure_OperationNotAllowed(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)

	repeatScenarioInParallel(t, func() {
		givenDeviceUniqueID := uuid.NewString()
		givenUserID := shouldGenerateRandomUserID(t)

		arg := request{reqUserID: givenUserID, reqDeviceID: givenDeviceUniqueID, reqHeaders: []http.Header{authHeader()}}
		resp, status, headers := whenDoGetUserDevices(ctx, t, &arg)

		expectedRespErr := fmt.Sprintf(`operation not allowed. uri>%v!=token>%v`, givenUserID, testUserID)
		expectedRespCode := `OPERATION_NOT_ALLOWED`

		resp.thenExpectResponse(t, expectedRespCode, expectedRespErr)
		thenExpectStatus(t, status, http.StatusForbidden)
		thenExpectHeaders(t, headers)
	})
}

func whenDoGetUserDevices(ctx context.Context, tb testing.TB, arg *request) (resp *response, status int, headers http.Header) {
	tb.Helper()
	body, status, headers := serverConnector.Get(ctx, tb, fmt.Sprintf(`/v1r/users/%v/devices/%v/settings`, arg.reqUserID, arg.reqDeviceID), arg.reqHeaders...)
	resp = new(response)
	require.NoError(tb, json.Unmarshal([]byte(body), resp))

	return
}

func (r *response) thenExpectResponse(tb testing.TB, expectedRespCode, expectedRespErr string) {
	tb.Helper()
	assert.Equal(tb, r.Code, expectedRespCode)
	assert.Equal(tb, r.Error, expectedRespErr)
}

func thenExpectStatus(tb testing.TB, status, expectedStatus int) {
	tb.Helper()
	assert.Equal(tb, expectedStatus, status)
}

func thenExpectHeaders(tb testing.TB, headers http.Header) {
	tb.Helper()
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

func shouldGenerateRandomUserID(t *testing.T) string {
	t.Helper()

	return "did:ethr:0x" + randomHex(t, 20)
}

func repeatScenarioInParallel(t *testing.T, f func()) {
	t.Helper()
	for i := 0; i < repeatNum; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			f()
		})
	}
}
