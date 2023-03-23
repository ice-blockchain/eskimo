// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/ice-blockchain/wintr/testing"
)

//nolint:paralleltest // snake_case is ok for testing; we can't parallelize it because we have a limit number of real auth tokens.
func TestService_DeleteUser_Success_Deleted(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, token := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we create an user", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID, token, "bogus.success11")
	})
	var body string
	var status int
	WHEN("deleting the user", func() {
		body, status = bridge.DeleteUser(ctx, t, userID, token)
	})
	THEN(func() {
		IT("is is deleted", func() {
			assert.Equal(t, 200, status)
		})
		IT("returns nothing", func() {
			assert.Empty(t, body)
		})
	})
}

//nolint:paralleltest // snake_case is ok for testing; we can't parallelize it because we have a limit number of real auth tokens.
func TestService_DeleteUser_Success_NotDeleted(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, token := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we don't have any user", func() {})
	var body string
	var status int
	WHEN("deleting some non-existing user", func() {
		body, status = bridge.DeleteUser(ctx, t, userID, token)
	})
	THEN(func() {
		IT("is not deleted", func() {
			assert.Equal(t, 204, status)
		})
		IT("returns nothing", func() {
			assert.Empty(t, body)
		})
	})
}

//nolint:paralleltest // snake_case is ok for testing; we can't parallelize it because we have a limit number of real auth tokens.
func TestService_DeleteUser_Failure_NoUserID(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, token := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we create an user", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID, token, "bogus.failure11")
	})
	var body string
	var status int
	WHEN("deleting the user without specifying the userID", func() {
		body, status = bridge.DeleteUser(ctx, t, "", token)
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 404, status)
		})
		IT("returns specific error code and some error message", func() {
			assert.Equal(t, "404 page not found", body)
		})
	})
}

//nolint:paralleltest // snake_case is ok for testing; we can't parallelize it because we have a limit number of real auth tokens.
func TestService_DeleteUser_Failure_Unauthorized(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, token := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we create an user", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID, token, "bogus.failure12")
	})
	var body string
	var status int
	WHEN("deleting the user without being authorized", func() {
		body, status = bridge.DeleteUser(ctx, t, userID, "invalid token")
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 401, status)
		})
		IT("returns specific error code and some error message", func() {
			expected := `{"error":".+","code":"INVALID_TOKEN"}`
			bridge.AssertResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest // snake_case is ok for testing; we can't parallelize it because we have a limit number of real auth tokens.
func TestService_DeleteUser_Failure_Forbidden(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, token := bridge.GetTestingAuthorizationAt(0)
	_, differentToken := bridge.GetTestingAuthorizationAt(1)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we create an user", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID, token, "bogus.failure13")
	})
	var body string
	var status int
	WHEN("deleting the user with a different token", func() {
		body, status = bridge.DeleteUser(ctx, t, userID, differentToken)
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 403, status)
		})
		IT("returns specific error code and some error message", func() {
			expected := `{"error":".+","code":"OPERATION_NOT_ALLOWED"}`
			bridge.AssertResponseBody(t, expected, body)
		})
	})
}
