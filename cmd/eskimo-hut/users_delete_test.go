// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ice-blockchain/eskimo/users"
)

// nolint:revive,unparam // We need those arguments to verify result.
func testDeleteUser(ctx context.Context, tb testing.TB, userID users.UserID, expectedRespStatus int, extraHeaders ...map[string]string) {
	tb.Helper()
	reqHeaders := http.Header{}
	reqHeaders.Set("Authorization", fmt.Sprintf("Bearer %v", testMagicToken))
	if len(extraHeaders) > 0 {
		for _, header := range extraHeaders {
			for headerKey, headerValue := range header {
				reqHeaders.Set(headerKey, headerValue)
			}
		}
	}
	_, status, _ := serverConnector.Delete(ctx, tb, fmt.Sprintf(`/v1w/users/%v`, userID), reqHeaders)
	assert.Equal(tb, expectedRespStatus, status)
}
