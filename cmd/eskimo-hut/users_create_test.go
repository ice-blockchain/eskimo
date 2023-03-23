// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/ice-blockchain/eskimo/users"
	. "github.com/ice-blockchain/wintr/testing"
)

//nolint:paralleltest,dupl // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_CreateUser_Success_WithEmail(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, token := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("no other user exists that might generate a conflict", func() {})
	var body string
	var status int
	WHEN("creating a new user with `email` set", func() {
		reqBody := `{"email":"something@ice.io","username":"bogus.success1"}`
		body, status = bridge.CreateUser(ctx, t, userID, token, reqBody)
	})
	THEN(func() {
		IT("is successfully created", func() {
			assert.Equal(t, 201, status)
		})
		IT("responded with its default JSON body with the expected `email`", func() {
			expected := new(users.User)
			expected.ID = userID
			expected.ReferredBy = userID
			expected.Username = "bogus.success1"
			expected.Email = "something@ice.io"
			bridge.AssertCreateUserResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest,funlen // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_CreateUser_Success_WithReferral(t *testing.T) {
	if testing.Short() {
		return
	}
	userID1, token1 := bridge.GetTestingAuthorizationAt(0)
	userID2, token2 := bridge.GetTestingAuthorizationAt(1)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we have some existing user", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID1, token1, "bogus.success2")
	})
	var body string
	var status int
	WHEN("creating a new user with `referredBy` pointing to that existing user", func() {
		reqBody := fmt.Sprintf(`{"referredBy":%q,"username":"bogus.success3"}`, userID1)
		body, status = bridge.CreateUser(ctx, t, userID2, token2, reqBody)
	})
	THEN(func() {
		IT("is successfully created", func() {
			assert.Equal(t, 201, status)
		})
		IT("responded with its default JSON body with the expected `referredBy`", func() {
			expected := new(users.User)
			expected.ID = userID2
			expected.ReferredBy = userID1
			expected.Username = "bogus.success3"
			bridge.AssertCreateUserResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_CreateUser_Success_OnlyUsernameProvided(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, token := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("no other user exists that might generate a conflict", func() {})
	var body string
	var status int
	WHEN("creating a new user by providing only the `username`", func() {
		reqBody := `{"username":"Bo-gus.suc_cess4"}`
		body, status = bridge.CreateUser(ctx, t, userID, token, reqBody)
	})
	THEN(func() {
		IT("is successfully created", func() {
			assert.Equal(t, 201, status)
		})
		IT("responded with its default JSON body with the expected `username`", func() {
			expected := new(users.User)
			expected.ID = userID
			expected.ReferredBy = userID
			expected.Username = "bo-gus.suc_cess4"
			bridge.AssertCreateUserResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest,dupl // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_CreateUser_Success_WithPhoneNumber(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, token := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("no other user exists that might generate a conflict", func() {})
	var body string
	var status int
	WHEN("creating a new user with `phoneNumber` & `phoneNumberHash` set", func() {
		reqBody := `{"phoneNumber":"+1234567893","phoneNumberHash":"25f9e794323b453885f5181f1b624d0b","username":"bogus.success5"}`
		body, status = bridge.CreateUser(ctx, t, userID, token, reqBody)
	})
	THEN(func() {
		IT("is successfully created", func() {
			assert.Equal(t, 201, status)
		})
		IT("responded with its default JSON body with the expected `phoneNumber`", func() {
			expected := new(users.User)
			expected.ID = userID
			expected.ReferredBy = userID
			expected.Username = "bogus.success5"
			expected.PhoneNumber = "+1234567893"
			bridge.AssertCreateUserResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest,funlen // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_CreateUser_Success_WithEverythingSet(t *testing.T) {
	if testing.Short() {
		return
	}
	userID1, token1 := bridge.GetTestingAuthorizationAt(0)
	userID2, token2 := bridge.GetTestingAuthorizationAt(1)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we have some existing user", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID1, token1, "bogus.success6")
	})
	var body string
	var status int
	WHEN("creating a new user with every property set & referred by that existing user", func() {
		reqBody := fmt.Sprintf(`{"email":"something-else@ice.io",
										"referredBy":%q,
										"phoneNumber":"+1234567892",
										"phoneNumberHash":"5f5181f1b624d1b",
										"username":"bogus.success7"}`, userID1)
		body, status = bridge.CreateUser(ctx, t, userID2, token2, reqBody, "9.9.9.11")
	})
	THEN(func() {
		IT("is successfully created", func() {
			assert.Equal(t, 201, status)
		})
		IT("responded with its default JSON body with every property set", func() {
			expected := new(users.User)
			expected.ID = userID2
			expected.ReferredBy = userID1
			expected.Username = "bogus.success7"
			expected.Email = "something-else@ice.io"
			expected.PhoneNumber = "+1234567892"
			expected.Country = "CH"
			expected.City = "Zurich"
			bridge.AssertCreateUserResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_CreateUser_Failure_Unauthorized(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, _ := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("no other user exists that might generate a conflict", func() {})
	var body string
	var status int
	WHEN("creating an user without being authorized", func() {
		reqBody := `{"username": "bogus.failure1"}`
		body, status = bridge.CreateUser(ctx, t, userID, "invalid token", reqBody)
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 401, status)
		})
		IT("returns specific error code and some error message", func() {
			expected := `{"error":".+","code":"INVALID_TOKEN"}` //nolint:goconst // Nope, we need to be descriptive.
			bridge.AssertResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_CreateUser_Failure_InvalidStructure(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, token := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("no other user exists that might generate a conflict", func() {})
	var body string
	var status int
	WHEN("creating an user invalid body structure", func() {
		reqBody := `{"username":2,"email":2,"referredBy":2,"phoneNumber":2,"phoneNumberHash":2}`
		body, status = bridge.CreateUser(ctx, t, userID, token, reqBody)
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 422, status)
		})
		IT("returns specific error code and some error message", func() {
			expected := `{"error":".+","code":"STRUCTURE_VALIDATION_FAILED"}`
			bridge.AssertResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_CreateUser_Failure_PhoneNumberRequiredIfPhoneNumberHashIsSet(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, token := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("no other user exists that might generate a conflict", func() {})
	var body string
	var status int
	WHEN("creating an user with `phoneNumberHash` but no `phoneNumber`", func() {
		reqBody := `{"username":"bogus.failure2","phoneNumberHash":"25f9e794323b453885f5181f1b624d1b"}`
		body, status = bridge.CreateUser(ctx, t, userID, token, reqBody)
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 422, status)
		})
		IT("returns specific error code and some error message", func() {
			expected := `{"error":".*phoneNumber.*phoneNumberHash.*","code":"INVALID_PROPERTIES"}` //nolint:goconst // Nope, we need to be descriptive.
			bridge.AssertResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_CreateUser_Failure_PhoneNumberHashRequiredIfPhoneNumberIsSet(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, token := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("no other user exists that might generate a conflict", func() {})
	var body string
	var status int
	WHEN("creating an user with `phoneNumber` but no `phoneNumberHash`", func() {
		reqBody := `{"username":"bogus.failure3","phoneNumber":"+1234567891"}`
		body, status = bridge.CreateUser(ctx, t, userID, token, reqBody)
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 422, status)
		})
		IT("returns specific error code and some error message", func() {
			expected := `{"error":".*phoneNumber.*phoneNumberHash.*","code":"INVALID_PROPERTIES"}`
			bridge.AssertResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_CreateUser_Failure_NoUsername(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, token := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("no other user exists that might generate a conflict", func() {})
	var body string
	var status int
	WHEN("creating an user with no `username`", func() {
		reqBody := `{}`
		body, status = bridge.CreateUser(ctx, t, userID, token, reqBody)
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 422, status)
		})
		IT("returns specific error code and some error message", func() {
			expected := `{"error":".+Username.+","code":"MISSING_PROPERTIES"}` //nolint:goconst // Nope, we need to be descriptive.
			bridge.AssertResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest,funlen // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_CreateUser_Failure_WithEverythingSetExceptUsername(t *testing.T) {
	if testing.Short() {
		return
	}
	userID1, token1 := bridge.GetTestingAuthorizationAt(0)
	userID2, token2 := bridge.GetTestingAuthorizationAt(1)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we have some existing user", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID1, token1, "bogus.failure10")
	})
	var body string
	var status int
	WHEN("creating a new user every property set, except `username`", func() {
		reqBody := fmt.Sprintf(`{"email":"something-else-failed@ice.io",
										"referredBy":%q,
										"phoneNumber":"+1234567890",
										"phoneNumberHash":"5f5181f1b624d2b"}`, userID1)
		body, status = bridge.CreateUser(ctx, t, userID2, token2, reqBody)
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 422, status)
		})
		IT("returns specific error code and some error message", func() {
			expected := `{"error":".+Username.+","code":"MISSING_PROPERTIES"}`
			bridge.AssertResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_CreateUser_Failure_InvalidUsername(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, token := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("no other user exists that might generate a conflict", func() {
	})
	var body string
	var status int
	for _, username := range allInvalidUsernames() {
		WHEN("creating an user with invalid `username`", func() {
			reqBody := fmt.Sprintf(`{"username":%q}`, username)
			body, status = bridge.CreateUser(ctx, t, userID, token, reqBody)
		})
		THEN(func() {
			IT("fails", func() {
				assert.Equal(t, 400, status, "for username %v %v", username, username[len(username)-1])
			})
			IT("returns specific error code and some error message", func() {
				expected := `{"error":".+","code":"INVALID_USERNAME"}`
				bridge.AssertResponseBody(t, expected, body)
			})
		})
	}
}

//nolint:gocognit,gocyclo,cyclop,revive // Those are just ASCII runes we don't want in an username.
func allInvalidUsernames() (invalidUsernames []string) {
	invalidUsernames = append(invalidUsernames, "aaa", "aaaaaaaaaaaaaaaaaaaaa")
	for ascii := 0; ascii <= int(byte(255)); ascii++ {
		isDot := ascii == 45
		isHyphen := ascii == 46
		isUnderscore := ascii == 95
		isDigit := ascii >= 48 && ascii <= 57
		isLetter := (ascii >= 65 && ascii <= 90) || (ascii >= 97 && ascii <= 122)
		specialCharThatBreaksURLPatternMatching := ascii == 11 || ascii == 127 || (ascii >= 14 && ascii <= 31) || (ascii >= 0 && ascii <= 7)
		if !isDot &&
			!isHyphen &&
			!isUnderscore &&
			!isDigit &&
			!isLetter &&
			!specialCharThatBreaksURLPatternMatching {
			invalidUsernames = append(invalidUsernames, "aaa"+string(rune(ascii)))
		}
	}

	return invalidUsernames
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_CreateUser_Failure_DuplicateUserID(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, token := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("an user already exists with the some ID", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID, token, "bogus.failure5")
	})
	var body string
	var status int
	WHEN("creating the same user but with different information", func() {
		reqBody := `{"username":"bogus.failure6"}`
		body, status = bridge.CreateUser(ctx, t, userID, token, reqBody)
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 409, status)
		})
		IT("returns specific error code, extra data and some error message", func() {
			expected := `{"data":{"field":"id"},"error":".+","code":"CONFLICT_WITH_ANOTHER_USER"}`
			bridge.AssertResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_CreateUser_Failure_DuplicateUserName(t *testing.T) {
	if testing.Short() {
		return
	}
	userID1, token1 := bridge.GetTestingAuthorizationAt(0)
	userID2, token2 := bridge.GetTestingAuthorizationAt(1)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("an different user already exists with the some username", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID1, token1, "bogus.failure8")
	})
	var body string
	var status int
	WHEN("creating a new user with the same `username`", func() {
		reqBody := `{"username":"bogus.failure8"}`
		body, status = bridge.CreateUser(ctx, t, userID2, token2, reqBody)
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 409, status)
		})
		IT("returns specific error code, extra data and some error message", func() {
			expected := `{"data":{"field":"username"},"error":".+","code":"CONFLICT_WITH_ANOTHER_USER"}`
			bridge.AssertResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_CreateUser_Failure_NonExistingReferral(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, token := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("no other user exists that might generate a conflict", func() {})
	var body string
	var status int
	WHEN("creating a new user with `referredBy` pointing to a non existent user", func() {
		reqBody := fmt.Sprintf(`{"referredBy":%q, "username":"bogus.failure9"}`, uuid.NewString())
		body, status = bridge.CreateUser(ctx, t, userID, token, reqBody)
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 404, status)
		})
		IT("returns specific error code and some error message", func() {
			expected := `{"error":".+","code":"REFERRAL_NOT_FOUND"}`
			bridge.AssertResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_CreateUser_Failure_SelfReferred(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, token := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("no other user exists that might generate a conflict", func() {})
	var body string
	var status int
	WHEN("creating a new user with `referredBy` pointing to the user itself", func() {
		reqBody := fmt.Sprintf(`{"referredBy":%q, "username":"bogus.failure10"}`, userID)
		body, status = bridge.CreateUser(ctx, t, userID, token, reqBody)
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 422, status)
		})
		IT("returns specific error code and some error message", func() {
			expected := `{"error":".+","code":"INVALID_PROPERTIES"}`
			bridge.AssertResponseBody(t, expected, body)
		})
	})
}
