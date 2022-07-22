// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nolint:nosnakecase // We're using this naming for tests with underscore
func TestService_CreateUser_Failure_Unauthorized(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	testCreateUser(ctx, t,
		`{"username": "test"}`,
		`{"error":"unexpected end of JSON input","code":"INVALID_TOKEN"}`,
		401,
		map[string]string{"Authorization": ""})
}

// nolint:nosnakecase // We're using this naming for tests with underscore
func TestService_CreateUser_Failure_NoPhoneHash(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	testCreateUser(ctx, t,
		`{"email": "testuser@example.com", "phoneNumber": "+123456789","phoneNumberHash": "","username": "test"}`,
		`{"error":"phoneNumber must be provided only together with phoneNumberHash","code":"INVALID_PROPERTIES"}`, 422)
}

// nolint:nosnakecase // We're using this naming for tests with underscore
func TestService_CreateUser_Failure_PhoneHashWithoutPhoneNumber(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	testCreateUser(ctx, t,
		`{"email": "testuser@example.com", "phoneNumber": "","phoneNumberHash": "25f9e794323b453885f5181f1b624d0b","username": "test"}`,
		`{"error":"phoneNumber must be provided only together with phoneNumberHash","code":"INVALID_PROPERTIES"}`, 422)
}

// nolint:nosnakecase // We're using this naming for tests with underscore
func TestService_CreateUser_Failure_NoUsername(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	testCreateUser(ctx, t,
		`{"email": "testuser@example.com", "phoneNumber": "+123456789","phoneNumberHash": "25f9e794323b453885f5181f1b624d0b"}`,
		"{\"error\":\"properties `Username` are required\",\"code\":\"MISSING_PROPERTIES\"}", 422)
}

// nolint:nosnakecase // We're using this naming for tests with underscore
func TestService_CreateUser_Failure_NonExistingReferredBy(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	testCreateUser(ctx, t,
		`{"email": "testuser@example.com","username": "test", "referredBy":"NON_EXISTING_USER"}`,
		`REFERRAL_NOT_FOUND`, 404)
}

// nolint:nosnakecase // We're using this naming for tests with underscore
func TestService_CreateUser_Failure_UsernameIsTooShort(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	testCreateUser(ctx, t,
		`{"email": "testuser@example.com", "username": "t"}`,
		`{"error":"username: t is invalid, it should match regex: \^\[\-\\\\w\.\]\{4,20\}\$","code":"INVALID_USERNAME"}`, 400)
}

// nolint:nosnakecase // We're using this naming for tests with underscore
func TestService_CreateUser_Failure_UsernameIsTooLong(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	testCreateUser(ctx, t,
		`{"email": "testuser@example.com", "username": "veryVeryLongUsernameWithMoreThan20Symbols"}`,
		`{"error":"username: veryVeryLongUsernameWithMoreThan20Symbols is invalid, it should match regex: \^\[\-\\\\w\.\]\{4,20\}\$","code":"INVALID_USERNAME"}`,
		400)
}

// nolint:nosnakecase // We're using this naming for tests with underscore
func TestService_CreateUser_Failure_UsernameWithSpecialCharacters(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	t.Cleanup(cancel)
	specCharacters := "!#%',:;=@`|~"
	for _, character := range specCharacters {
		c := character
		t.Run("test special character "+string(character), func(t *testing.T) {
			t.Parallel()
			testCreateUser(ctx, t,
				fmt.Sprintf(`{"email": "testuser@example.com", "username": "user%v"}`, string(c)),
				fmt.Sprintf(`{"error":"username: user%v is invalid, it should match regex: \^\[\-\\\\w\.\]\{4,20\}\$","code":"INVALID_USERNAME"}`, string(c)), 400)
		})
	}
}

// nolint:nosnakecase,funlen,paralleltest // We're using this naming for tests with underscore
func TestService_CreateUser_Failure_DuplicateUserID(t *testing.T) {
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	// User creation -> 201.
	testCreateUser(ctx, t,
		`{"email": "testuser@example.com", "username": "test"}`,
		fmt.Sprintf(`{
			"createdAt":%[1]q,
			"updatedAt":%[1]q,
			"id":"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2",
			"username":"test",
			"profilePictureUrl":"https://ice-staging.b-cdn.net/profile/default-user-image.jpg",
			"country":"-",
			"city":"This is DB24 demo BIN database. Please evaluate IP address from 0.0.0.0 to 99.255.255.255.",
			"email":"testuser@example.com"
		}`, timeRegex),
		201)
	// Duplicate userID (cuz of the same auth header) -> 409.
	testCreateUser(ctx, t,
		`{"email": "testuser@example.com","username": "another_user_test"}`,
		fmt.Sprintf(`{
			"data":{"field":"id"},
			"error":"failed to create user \\u0026main.CreateUserArg{ReferredBy:\\"\\", Username:\\"another_user_test\\", PhoneNumber:\\"\\", PhoneNumberHash:\\"\\", Email:\\"testuser@example.com\\"}: 
				failed to insert user {
				\\"createdAt\\":%[1]s,
				\\"updatedAt\\":%[1]s,
				\\"id\\":\\"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2\\",
				\\"username\\":\\"another_user_test\\",
				\\"profilePictureUrl\\":\\"default-user-image.jpg\\",
				\\"country\\":\\"-\\",
				\\"city\\":\\"This is DB24 demo BIN database. Please evaluate IP address from 0.0.0.0 to 99.255.255.255.\\",
				\\"email\\":\\"testuser@example.com\\"
			}: duplicate","code":"CONFLICT_WITH_ANOTHER_USER"
		}`,
			strings.ReplaceAll(fmt.Sprintf("%q", timeRegex), `"`, "\\\\\"")),
		409)
	testDeleteUser(ctx, t, "did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2", 200)
}

// nolint:nosnakecase,funlen,paralleltest // We're using this naming for tests with underscore
func TestService_CreateUser_Failure_Duplicate(t *testing.T) {
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	// User creation -> 201.
	testCreateUser(ctx, t,
		`{"email": "testuser@example.com", "username": "test"}`,
		fmt.Sprintf(`{
			"createdAt":%[1]q,
			"updatedAt":%[1]q,
			"id":"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2",
			"username":"test",
			"profilePictureUrl":"https://ice-staging.b-cdn.net/profile/default-user-image.jpg",
			"country":"-",
			"city":"This is DB24 demo BIN database. Please evaluate IP address from 0.0.0.0 to 99.255.255.255.",
			"email":"testuser@example.com"
		}`, timeRegex),
		201)
	// Duplicate user name -> 409.
	testCreateUser(ctx, t,
		`{"email": "testuser@example.com","username": "test"}`,
		fmt.Sprintf(`{
			"data":{"field":"username"},
			"error":"failed to create user \\u0026main.CreateUserArg{ReferredBy:\\"\\", Username:\\"test\\", PhoneNumber:\\"\\", PhoneNumberHash:\\"\\", Email:\\"testuser@example.com\\"}: 
				failed to insert user {
				\\"createdAt\\":%[1]s,
				\\"updatedAt\\":%[1]s,
				\\"id\\":\\"did:ethr:0xDeb2A20363E9063ad521B3156304b9E12834644B\\",
				\\"username\\":\\"test\\",
				\\"profilePictureUrl\\":\\"default-user-image.jpg\\",
				\\"country\\":\\"-\\",
				\\"city\\":\\"This is DB24 demo BIN database. Please evaluate IP address from 0.0.0.0 to 99.255.255.255.\\",
				\\"email\\":\\"testuser@example.com\\"
			}: duplicate","code":"CONFLICT_WITH_ANOTHER_USER"
		}`, strings.ReplaceAll(fmt.Sprintf("%q", timeRegex), `"`, "\\\\\"")),
		// Another token to create user with another userID but same username.
		409, map[string]string{"Authorization": fmt.Sprintf("Bearer %v", testMagicToken2ndUser)})
	testDeleteUser(ctx, t, "did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2", 200)
}

// nolint:nosnakecase,paralleltest // We're using this naming for tests with underscore
func TestService_CreateUser_Success_WithEmail(t *testing.T) {
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	// User creation -> 201.
	testCreateUser(ctx, t,
		`{"email": "testuser@example.com", "username": "test"}`,
		fmt.Sprintf(`{
			"createdAt":%[1]q,
			"updatedAt":%[1]q,
			"id":"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2",
			"username":"test",
			"profilePictureUrl":"https://ice-staging.b-cdn.net/profile/default-user-image.jpg",
			"country":"-",
			"city":"This is DB24 demo BIN database. Please evaluate IP address from 0.0.0.0 to 99.255.255.255.",
			"email":"testuser@example.com"
		}`, timeRegex),
		201)
	testDeleteUser(ctx, t, "did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2", 200)
}

// nolint:nosnakecase,funlen,paralleltest // We're using this naming for tests with underscore
func TestService_CreateUser_Success_WithReferral(t *testing.T) {
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	// T0.
	testCreateUser(ctx, t,
		`{"email": "testuser@example.com", "username": "test"}`,
		fmt.Sprintf(`{
			"createdAt":%[1]q,
			"updatedAt":%[1]q,
			"id":"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2",
			"username":"test",
			"profilePictureUrl":"https://ice-staging.b-cdn.net/profile/default-user-image.jpg",
			"country":"-",
			"city":"This is DB24 demo BIN database. Please evaluate IP address from 0.0.0.0 to 99.255.255.255.",
			"email":"testuser@example.com"
		}`, timeRegex),
		201)
	// Referred user.
	testCreateUser(ctx, t,
		`{"referredBy": "did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2", "username": "test_referred"}`,
		fmt.Sprintf(`{
			"createdAt":%[1]q,
			"updatedAt":%[1]q,
			"id":"did:ethr:0xDeb2A20363E9063ad521B3156304b9E12834644B",
			"username":"test_referred",
			"profilePictureUrl":"https://ice-staging.b-cdn.net/profile/default-user-image.jpg",
			"country":"-",
			"city":"This is DB24 demo BIN database. Please evaluate IP address from 0.0.0.0 to 99.255.255.255.",
			"referredBy":"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"
		}`, timeRegex),
		201, map[string]string{"Authorization": fmt.Sprintf("Bearer %v", testMagicToken2ndUser)}) // Another token to create from another user.
	testDeleteUser(ctx, t, "did:ethr:0xDeb2A20363E9063ad521B3156304b9E12834644B", 200,
		map[string]string{"Authorization": fmt.Sprintf("Bearer %v", testMagicToken2ndUser)},
	)
	testDeleteUser(ctx, t, "did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2", 200)
}

// nolint:nosnakecase // We're using this naming for tests with underscore
func TestService_CreateUser_Failure_NonExistingReferral(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	testCreateUser(ctx, t,
		`{"referredBy": "did:ethr:NON_EXISTING_USER", "username": "test"}`,
		fmt.Sprintf(`{
			"error":"failed to create user \\u0026main.CreateUserArg{ReferredBy:\\"did:ethr:NON_EXISTING_USER\\", Username:\\"test\\", PhoneNumber:\\"\\", PhoneNumberHash:\\"\\", Email:\\"\\"}: 
			failed to insert user {
				\\"createdAt\\":%[1]s,
				\\"updatedAt\\":%[1]s,
				\\"id\\":\\"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2\\",
				\\"username\\":\\"test\\",
				\\"profilePictureUrl\\":\\"default-user-image.jpg\\",
				\\"country\\":\\"-\\",
				\\"city\\":\\"This is DB24 demo BIN database. Please evaluate IP address from 0.0.0.0 to 99.255.255.255.\\",
				\\"referredBy\\":\\"did:ethr:NON_EXISTING_USER\\"
			}: relation not found","code":"REFERRAL_NOT_FOUND"
		}`,
			strings.ReplaceAll(fmt.Sprintf("%q", timeRegex), `"`, "\\\\\"")),
		404)
}

// nolint:nosnakecase // We're using this naming for tests with underscore
func TestService_CreateUser_Failure_SelfReferred(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	testCreateUser(ctx, t,
		`{"referredBy": "did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2", "username": "test"}`,
		`{"error":"you cannot use yourself as your own referral","code":"INVALID_PROPERTIES"}`,
		422)
}

// nolint:nosnakecase,paralleltest // We're using this naming for tests with underscore
func TestService_CreateUser_Success(t *testing.T) {
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), testDeadline)
	defer cancel()
	// User creation -> 201.
	testCreateUser(ctx, t,
		`{"username": "test_no_email"}`,
		fmt.Sprintf(`{
			"createdAt":%[1]q,
			"updatedAt":%[1]q,
			"id":"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2",
			"username":"test_no_email",
			"profilePictureUrl":"https://ice-staging.b-cdn.net/profile/default-user-image.jpg",
			"country":"-",
			"city":"This is DB24 demo BIN database. Please evaluate IP address from 0.0.0.0 to 99.255.255.255."
		}`, timeRegex),
		201)
	testDeleteUser(ctx, t, "did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2", 200)
}

// nolint:revive // We need those arguments to verify result.
func testCreateUser(ctx context.Context, tb testing.TB, reqBody, expectedRespBody string, expectedRespStatus int, extraHeaders ...map[string]string) {
	tb.Helper()
	jsonBody, contentType := serverConnector.WrapJSONBody(reqBody)
	reqHeaders := http.Header{}
	reqHeaders.Set("Content-Type", contentType)
	reqHeaders.Set("Authorization", fmt.Sprintf("Bearer %v", testMagicToken))
	if len(extraHeaders) > 0 {
		for _, header := range extraHeaders {
			for headerKey, headerValue := range header {
				reqHeaders.Set(headerKey, headerValue)
			}
		}
	}
	body, status, headers := serverConnector.Post(ctx, tb, `/v1w/users`, jsonBody, reqHeaders)
	expectedRespBody = strings.ReplaceAll(strings.ReplaceAll(expectedRespBody, "\t", ""), "\n", "")
	assert.Regexp(tb, regexp.MustCompile(expectedRespBody), body)
	assert.Equal(tb, expectedRespStatus, status)
	l, err := strconv.Atoi(headers.Get("Content-Length"))
	require.NoError(tb, err)
	assert.Greater(tb, l, 0)
	headers.Del("Date")
	headers.Del("Content-Length")
	assert.Equal(tb, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, headers)
}
