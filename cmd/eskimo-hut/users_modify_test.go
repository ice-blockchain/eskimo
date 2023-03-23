// SPDX-License-Identifier: ice License 1.0

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"

	"github.com/ice-blockchain/eskimo/users"
	. "github.com/ice-blockchain/wintr/testing"
)

//nolint:paralleltest,funlen // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_ModifyUser_Success_CommonFields(t *testing.T) {
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	userID, authToken := bridge.GetTestingAuthorizationAt(0)
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we have a user", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID, authToken, "bogus.username.0")
	})
	var body string
	var status int
	WHEN("we're trying to modify common fields of the user", func() {
		body, status = modifyUser(ctx, t, userID, authToken, map[string]any{
			"firstName":               "Test Change",
			"lastName":                "User's Name",
			"username":                "test1",
			"agendaPhoneNumberHashes": "8ec5e4255b35b140fd2c2c6ec4a02de315a774a4",
			"country":                 "GB",
			"city":                    "London",
		})
	})
	THEN(func() {
		IT("should return status 200", func() {
			assert.Equal(t, 200, status)
		})
		IT("should contain user information with updated fields", func() {
			expected := new(users.User)
			expected.ID = userID
			expected.LastName = "User's Name"
			expected.FirstName = "Test Change"
			expected.Username = "test1"
			expected.AgendaPhoneNumberHashes = "8ec5e4255b35b140fd2c2c6ec4a02de315a774a4"
			expected.Country = "GB"
			expected.City = "London"
			assertModifyUserResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest,funlen // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_ModifyUser_Success_UploadProfilePicture(t *testing.T) {
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	userID, authToken := bridge.GetTestingAuthorizationAt(0)
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we have the user with default profile picture", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID, authToken, "bogus.username.1")
	})
	var body string
	var status int
	WHEN("we try to upload new profile picture", func() {
		body, status = modifyUserProfilePicture(ctx, t, userID, authToken)
	})
	THEN(func() {
		updatedUser := new(users.User)
		IT("is successfully updated", func() {
			assert.Equal(t, 200, status)
		})
		IT("was uploaded on server", func() {
			require.NoError(t, json.UnmarshalContext(ctx, []byte(body), updatedUser))
			assertProfilePictureUploaded(ctx, t, updatedUser.ProfilePictureURL)
		})
		IT("has correct data in response body", func() {
			expected := new(users.User)
			expected.ID = userID
			expected.Username = "bogus.username.1"
			expected.ProfilePictureURL = updatedUser.ProfilePictureURL
			assertModifyUserResponseBody(t, expected, body)
		})
	})
}

//nolint:paralleltest,dupl // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_ModifyUser_Failure_NonExistingCountry(t *testing.T) {
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	userID, authToken := bridge.GetTestingAuthorizationAt(0)
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we have the user", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID, authToken, "bogus.username.3")
	})
	var body string
	var status int
	WHEN("we try to modify user and provide non-existing country", func() {
		body, status = modifyUser(ctx, t, userID, authToken, map[string]any{
			"country": "NON_EXISTING_COUNTRY",
			"city":    "New York City",
		})
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 400, status)
		})
		IT("returns error code and message", func() {
			bridge.AssertResponseBody(t, `{"error":"invalid country NON_EXISTING_COUNTRY","code":"INVALID_PROPERTIES"}`, body)
		})
	})
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_ModifyUser_Failure_MismatchedCity(t *testing.T) {
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	userID, authToken := bridge.GetTestingAuthorizationAt(0)
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we have the user", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID, authToken, "bogus.username.51")
	})
	var body string
	var status int
	WHEN("we try to update country without city", func() {
		body, status = modifyUser(ctx, t, userID, authToken, map[string]any{
			"country": "RU",
		})
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 400, status)
		})
		IT("returns error code and message", func() {
			bridge.AssertResponseBody(t, `{"error":".+","code":"INVALID_PROPERTIES"}`, body)
		})
	})
}

//nolint:paralleltest,funlen // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_ModifyUser_Failure_PhoneNumberNoHash(t *testing.T) {
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	userID, authToken := bridge.GetTestingAuthorizationAt(0)
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we have the user", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID, authToken, "bogus.username.4")
	})
	var body string
	var status int
	WHEN("we try to update phoneNumber without phoneNumberHash", func() {
		body, status = modifyUser(ctx, t, userID, authToken, map[string]any{
			"phoneNumber": "+1987654321",
		})
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 422, status)
		})
		IT("returns error code and message", func() {
			bridge.AssertResponseBody(t, `{"error":"phoneNumber.+phoneNumberHash.*","code":"INVALID_PROPERTIES"}`, body)
		})
	})
	WHEN("we try to update phoneNumberHash without phoneNumber", func() {
		body, status = modifyUser(ctx, t, userID, authToken, map[string]any{
			"phoneNumberHash": "Hash value",
		})
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 422, status)
		})
		IT("returns error code and message", func() {
			bridge.AssertResponseBody(t, `{"error":"phoneNumber.+phoneNumberHash.*","code":"INVALID_PROPERTIES"}`, body)
		})
	})
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_ModifyUser_Failure_InvalidPhoneFormat(t *testing.T) {
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	userID, authToken := bridge.GetTestingAuthorizationAt(0)
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we have the user", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID, authToken, "bogus.username.5")
	})
	var body string
	var status int
	WHEN("we try to update phoneNumber with invalid format", func() {
		body, status = modifyUser(ctx, t, userID, authToken, map[string]any{
			"phoneNumber":     strings.ReplaceAll(smsfixture.TestingPhoneNumber(2), "+", ""),
			"phoneNumberHash": "HashValue",
		})
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 400, status)
		})
		IT("returns error code and message", func() {
			expectedBody := `{"data":{"phoneNumber":".+"},"error":".+","code":"INVALID_PHONE_NUMBER_FORMAT"}`
			bridge.AssertResponseBody(t, expectedBody, body)
		})
	})
}

//nolint:paralleltest,dupl // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_ModifyUser_Failure_InvalidPhoneNumber(t *testing.T) {
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	userID, authToken := bridge.GetTestingAuthorizationAt(0)
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we have the user", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID, authToken, "bogus.username.6")
	})
	var body string
	var status int
	WHEN("we try to update phoneNumber with invalid phone", func() {
		body, status = modifyUser(ctx, t, userID, authToken, map[string]any{
			"phoneNumber":     "+7000123456789", // +7 -> 0 is unassigned to any country according to https://en.wikipedia.org/wiki/List_of_country_calling_codes
			"phoneNumberHash": "HashValue",
		})
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 400, status)
		})
		IT("returns error code and message", func() {
			bridge.AssertResponseBody(t, `{"error":".+","code":"INVALID_PHONE_NUMBER"}`, body)
		})
	})
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_ModifyUser_Failure_NoValues(t *testing.T) {
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	userID, authToken := bridge.GetTestingAuthorizationAt(0)
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we have the user", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID, authToken, "bogus.username.7")
	})
	var body string
	var status int
	WHEN("we try to modify user with no values", func() {
		body, status = modifyUser(ctx, t, userID, authToken, map[string]any{})
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 422, status)
		})
		IT("returns error code and description", func() {
			bridge.AssertResponseBody(t, `{"error":"modify request without values","code":"INVALID_PROPERTIES"}`, body)
		})
	})
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_ModifyUser_Failure_AnotherUser(t *testing.T) {
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	userID, authToken := bridge.GetTestingAuthorizationAt(0)
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we have the user", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID, authToken, "bogus.username.8")
	})
	var body string
	var status int
	WHEN("we trying to modify user authenticated as another user", func() {
		_, anotherToken := bridge.GetTestingAuthorizationAt(1)
		body, status = modifyUser(ctx, t, userID, anotherToken, map[string]any{
			"firstName": "Edited firstName",
		})
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 403, status)
		})
		IT("returns error code and description", func() {
			bridge.AssertResponseBody(t, `{"error":".+","code":"OPERATION_NOT_ALLOWED"}`, body)
		})
	})
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_ModifyUser_Failure_NonExistingUser(t *testing.T) {
	if testing.Short() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	userID, authToken := bridge.GetTestingAuthorizationAt(0)
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we have no existing users", func() {})
	var body string
	var status int
	WHEN("we trying to modify non-existing user", func() {
		body, status = modifyUser(ctx, t, userID, authToken, map[string]any{
			"email": "non-existing-user@ice.io",
		})
	})
	THEN(func() {
		IT("returns status 404", func() {
			assert.Equal(t, 404, status)
		})
		IT("returns error code USER_NOT_FOUND", func() {
			bridge.AssertResponseBody(t, `{"error":".+","code":"USER_NOT_FOUND"}`, body)
		})
	})
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_ModifyUser_Failure_InvalidUsername(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, authToken := bridge.GetTestingAuthorizationAt(0)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we have the user", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID, authToken, "bogus.username.9")
	})
	var body string
	var status int
	for _, username := range allInvalidUsernames() {
		WHEN("modifying an user with invalid `username`", func() {
			body, status = modifyUser(ctx, t, userID, authToken, map[string]any{
				"username": username,
			})
		})
		THEN(func() {
			IT("fails", func() {
				assert.Equal(t, 400, status, "for username %v", username)
			})
			IT("returns specific error code and some error message", func() {
				bridge.AssertResponseBody(t, `{"error":".+","code":"INVALID_USERNAME"}`, body)
			})
		})
	}
}

//nolint:paralleltest // We can't parallelize it because we have a limit number of real auth tokens.
func TestService_ModifyUser_Failure_DuplicatedByUsername(t *testing.T) {
	if testing.Short() {
		return
	}
	userID, authToken := bridge.GetTestingAuthorizationAt(0)
	duplicateUserID, duplicateAuthToken := bridge.GetTestingAuthorizationAt(1)
	ctx, cancel := context.WithTimeout(context.Background(), bridge.TestDeadline)
	defer cancel()
	SETUP("we cleanup existing users, if any", func() {
		bridge.MustDeleteAllUsers(ctx, t)
	})
	GIVEN("we have 2 users", func() {
		bridge.MustCreateDefaultUser(ctx, t, userID, authToken, "bogus.username.10")
		bridge.MustCreateDefaultUserWithReferredBy(ctx, t, duplicateUserID, duplicateAuthToken, "bogus.username.11", userID)
	})
	var body string
	var status int
	WHEN("modifying an user with duplicated username", func() {
		body, status = modifyUser(ctx, t, duplicateUserID, duplicateAuthToken, map[string]any{
			"username": "bogus.username.10",
		})
	})
	THEN(func() {
		IT("fails", func() {
			assert.Equal(t, 409, status)
		})
		IT("returns specific error code and some error message", func() {
			bridge.AssertResponseBody(t, `{"data":{"field":"username"},"error":".+","code":"CONFLICT_WITH_ANOTHER_USER"}`, body)
		})
	})
}

//nolint:revive,funlen,gocognit // It's more descriptive this way.
func modifyUser(ctx context.Context, tb testing.TB, userID, token string, reqBody map[string]any) (body string, status int) {
	tb.Helper()

	getBodyBefore, getStatusBefore := bridge.GetUser(ctx, tb, userID, token)

	clientIP := bridge.DefaultClientIP
	multipartBody, contentType := bridge.W.WrapMultipartBody(tb, reqBody)
	body, status, headers := bridge.W.Patch(ctx, tb, fmt.Sprintf(`/v1w/users/%v`, userID), multipartBody, bridge.RequestHeaders(token, contentType, clientIP))
	l, err := strconv.Atoi(headers.Get("Content-Length"))
	require.NoError(tb, err)
	assert.NotEmpty(tb, body)
	assert.Greater(tb, l, 0)
	headers.Del("Date")
	headers.Del("Content-Length")
	assert.Equal(tb, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, headers)

	getBodyAfter, getStatusAfter := bridge.GetUser(ctx, tb, userID, token)
	if status == 200 { //nolint:nestif // Lesser evil.
		assert.Equal(tb, status, getStatusAfter)
		assert.Equal(tb, body, getBodyAfter[:strings.LastIndex(getBodyAfter, ",\"referralCount\":")]+"}")
		assert.Equal(tb, getStatusBefore, getStatusAfter)
		if (len(reqBody) == 2 && reqBody["phoneNumber"] != nil && reqBody["phoneNumberHash"] != nil) ||
			(len(reqBody) == 1 && reqBody["email"] != nil) {
			assert.Equal(tb, getBodyBefore, getBodyAfter)
		} else {
			assert.NotEqual(tb, getBodyBefore, getBodyAfter)
		}
	} else {
		assert.Equal(tb, getStatusBefore, getStatusAfter)
		assert.Equal(tb, getBodyBefore, getBodyAfter)
		if getStatusBefore == 404 {
			if status != 403 {
				assert.Equal(tb, status, getStatusAfter)
			}
			bridge.AssertResponseBody(tb, `{"error":".+","code":"USER_NOT_FOUND"}`, getBodyAfter)
		}
	}
	if getStatusBefore == 401 {
		assert.Equal(tb, 401, status)
	}

	return body, status
}

func modifyUserProfilePicture(ctx context.Context, tb testing.TB, userID, token string) (body string, status int) {
	tb.Helper()
	pic, err := profilePictures.Open(".testdata/profilePic.jpg")
	require.NoError(tb, err)
	defer require.NoError(tb, pic.Close())

	return modifyUser(ctx, tb, userID, token, map[string]any{"profilePicture": pic})
}

func assertProfilePictureUploaded(ctx context.Context, tb testing.TB, url string) {
	tb.Helper()
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	require.NoError(tb, err)
	//nolint:gosec // Skip checking cert chain from CDN
	client := &http.Client{Transport: &http2.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	resp, err := client.Do(r)
	defer func() {
		require.NoError(tb, resp.Body.Close())
	}()
	require.NoError(tb, err)
	assert.Equal(tb, 200, resp.StatusCode)
	b, err := io.ReadAll(resp.Body)
	require.NoError(tb, err)
	assert.Greater(tb, len(b), 0)
}

//nolint:funlen // A lot of ifs & json, looks better like this.
func assertModifyUserResponseBody(tb testing.TB, expectedUsr *users.User, actualResponseBody string) {
	tb.Helper()
	var firstNameKV, lastNameKV, phoneNumberKV, emailKV, agendaKV string
	if expectedUsr.FirstName != "" {
		firstNameKV = fmt.Sprintf(`,"firstName":%q`, strings.ReplaceAll(expectedUsr.FirstName, ".", "[.]"))
	}
	if expectedUsr.LastName != "" {
		lastNameKV = fmt.Sprintf(`,"lastName":%q`, strings.ReplaceAll(expectedUsr.LastName, ".", "[.]"))
	}
	if expectedUsr.PhoneNumber != "" {
		phoneNumberKV = fmt.Sprintf(`,"phoneNumber":%q`, strings.ReplaceAll(expectedUsr.PhoneNumber, "+", "[+]"))
	}
	if expectedUsr.Email != "" {
		emailKV = fmt.Sprintf(`,"email":%q`, strings.ReplaceAll(strings.ReplaceAll(expectedUsr.Email, ".", "[.]"), "+", "[+]"))
	}
	if expectedUsr.AgendaPhoneNumberHashes != "" {
		agendaKV = fmt.Sprintf(`,"agendaPhoneNumberHashes":%q`, strings.ReplaceAll(expectedUsr.AgendaPhoneNumberHashes, ".", "[.]"))
	}
	if expectedUsr.Country == "" {
		expectedUsr.Country = bridge.DefaultClientIPCountry
	}
	if expectedUsr.City == "" {
		expectedUsr.City = bridge.DefaultClientIPCity
	} else {
		expectedUsr.City = strings.ReplaceAll(strings.ReplaceAll(expectedUsr.City, "+", "[+]"), ".", "[.]")
	}
	if expectedUsr.ProfilePictureURL == "" {
		expectedUsr.ProfilePictureURL = bridge.DefaultProfilePictureURL
	}
	username := strings.ReplaceAll(expectedUsr.Username, ".", "[.]")
	expectedResponseBody := fmt.Sprintf(`{
			"createdAt":%[1]q,
			"updatedAt":%[1]q,
			"id":%[2]q,
			"username":%[3]q
			%[4]v
			%[5]v
			%[6]v,
			"profilePictureUrl":%[7]q,
			"country":%[8]q,
			"city":%[9]q
			%[10]v,
			"referredBy":%[2]q
			%[11]v
		}`, bridge.TimeRegex, expectedUsr.ID, username, firstNameKV, lastNameKV, phoneNumberKV,
		expectedUsr.ProfilePictureURL, expectedUsr.Country, expectedUsr.City, emailKV, agendaKV)
	bridge.AssertResponseBody(tb, expectedResponseBody, actualResponseBody)
}
