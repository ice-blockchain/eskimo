// SPDX-License-Identifier: ice License 1.0

package fixture

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ice-blockchain/eskimo/users"
	serverfixture "github.com/ice-blockchain/wintr/server/fixture"
)

func NewBridge(read, write serverfixture.TestConnector) *TestConnectorsBridge {
	return &TestConnectorsBridge{
		R:                        read,
		W:                        write,
		TestDeadline:             testDeadline,
		TimeRegex:                timeRegex,
		DefaultClientIP:          defaultClientIP,
		DefaultClientIPCountry:   defaultClientIPCountry,
		DefaultClientIPCity:      defaultClientIPCity,
		DefaultProfilePictureURL: defaultProfilePictureURL,
	}
}

//nolint:revive,funlen // It's more descriptive this way.
func (b *TestConnectorsBridge) CreateUser(ctx context.Context, tb testing.TB, userID, token, reqBody string, clientIPs ...string) (body string, status int) {
	tb.Helper()

	getBodyBefore, getStatusBefore := b.GetUser(ctx, tb, userID, token)

	clientIP := defaultClientIP
	if len(clientIPs) != 0 {
		clientIP = clientIPs[0]
	}
	jsonBody, contentType := b.W.WrapJSONBody(reqBody)
	body, status, headers := b.W.Post(ctx, tb, `/v1w/users`, jsonBody, b.RequestHeaders(token, contentType, clientIP))
	l, err := strconv.Atoi(headers.Get("Content-Length"))
	require.NoError(tb, err)
	assert.NotEmpty(tb, body)
	assert.Greater(tb, l, 0)
	headers.Del("Date")
	headers.Del("Content-Length")
	assert.Equal(tb, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, headers)

	getBodyAfter, getStatusAfter := b.GetUser(ctx, tb, userID, token)
	if status == 201 { //nolint:gomnd // Nothing magical about it.
		assert.Equal(tb, 200, getStatusAfter) //nolint:gomnd // Nothing magical about it.
		assert.Equal(tb, body, getBodyAfter[:strings.LastIndex(getBodyAfter, ",\"referralCount\":")]+"}")
		assert.Equal(tb, 404, getStatusBefore) //nolint:gomnd // Nothing magical about it.
		b.AssertResponseBody(tb, `{"error":".+","code":"USER_NOT_FOUND"}`, getBodyBefore)
	} else {
		assert.Equal(tb, getStatusBefore, getStatusAfter)
		assert.Equal(tb, getBodyBefore, getBodyAfter)
		if getStatusBefore == 404 { //nolint:gomnd // Nothing magical about it.
			b.AssertResponseBody(tb, `{"error":".+","code":"USER_NOT_FOUND"}`, getBodyAfter)
		}
	}
	if getStatusBefore == 401 { //nolint:gomnd // Nothing magical about it.
		assert.Equal(tb, 401, status) //nolint:gomnd // Nothing magical about it.
	}

	return body, status
}

func (b *TestConnectorsBridge) DeleteUser(ctx context.Context, tb testing.TB, userID, token string) (body string, status int) {
	tb.Helper()

	getBodyBefore, getStatusBefore := b.GetUser(ctx, tb, userID, token)

	body, status, headers := b.W.Delete(ctx, tb, fmt.Sprintf(`/v1w/users/%v`, userID), b.RequestHeaders(token))
	headers.Del("Date")

	getBody, getStatus := b.GetUser(ctx, tb, userID, token)
	if status == 200 || status == 204 {
		assert.Empty(tb, body)
		if getStatusBefore == 200 { //nolint:gomnd // Nothing magical about it.
			assert.Equal(tb, http.Header{"Content-Length": []string{"0"}}, headers)
			assert.Equal(tb, 200, status) //nolint:gomnd // Nothing magical about it.
		}
		if getStatusBefore == 404 { //nolint:gomnd // Nothing magical about it.
			assert.Equal(tb, http.Header{}, headers)
			assert.Equal(tb, 204, status) //nolint:gomnd // Nothing magical about it.
		}
		assert.Equal(tb, 404, getStatus) //nolint:gomnd // Nothing magical about it.
		b.AssertResponseBody(tb, `{"error":".+","code":"USER_NOT_FOUND"}`, getBody)
	} else {
		assert.NotEmpty(tb, body)
		assert.Equal(tb, getStatusBefore, getStatus)
		assert.Equal(tb, getBodyBefore, getBody)
	}
	if getStatusBefore == 401 { //nolint:gomnd // Nothing magical about it.
		assert.Equal(tb, 401, status) //nolint:gomnd // Nothing magical about it.
	}

	return body, status
}

func (b *TestConnectorsBridge) GetUser(ctx context.Context, tb testing.TB, userID, token string) (body string, status int) {
	tb.Helper()

	body, status, headers := b.R.Get(ctx, tb, fmt.Sprintf(`/v1r/users/%v`, userID), b.RequestHeaders(token))
	l, err := strconv.Atoi(headers.Get("Content-Length"))
	require.NoError(tb, err)
	assert.NotEmpty(tb, body)
	assert.Greater(tb, l, 0)
	headers.Del("Date")
	headers.Del("Content-Length")
	assert.Equal(tb, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, headers)

	return body, status
}

func (*TestConnectorsBridge) RequestHeaders(token string, contentTypeOrClientIP ...string) http.Header {
	reqHeaders := http.Header{}
	reqHeaders.Set("Authorization", fmt.Sprintf("Bearer %v", token))
	reqHeaders.Set("cf-connecting-ip", defaultClientIP)
	for _, s := range contentTypeOrClientIP {
		if net.ParseIP(s) != nil {
			reqHeaders.Set("cf-connecting-ip", s)
		} else {
			reqHeaders.Set("Content-Type", s)
		}
	}

	return reqHeaders
}

//nolint:revive // Its more descriptive with a bit more args.
func (b *TestConnectorsBridge) MustCreateDefaultUser(ctx context.Context, tb testing.TB, userID, token, username string, clientIPs ...string) {
	tb.Helper()
	reqBody := fmt.Sprintf(`{"username":%q}`, username)
	body, status := b.CreateUser(ctx, tb, userID, token, reqBody, clientIPs...)
	assert.Equal(tb, 201, status) //nolint:gomnd // Nothing magical about it.

	expectedUsr := new(users.User)
	expectedUsr.ID = userID
	expectedUsr.ReferredBy = userID
	expectedUsr.Username = username
	b.AssertCreateUserResponseBody(tb, expectedUsr, body)
}

//nolint:revive // Its more descriptive with a bit more args.
func (b *TestConnectorsBridge) MustCreateDefaultUserWithReferredBy(ctx context.Context,
	tb testing.TB,
	userID, token, username, referredBy string,
	clientIPs ...string,
) {
	tb.Helper()
	reqBody := fmt.Sprintf(`{"username":%q, "referredBy":%q}`, username, referredBy)
	body, status := b.CreateUser(ctx, tb, userID, token, reqBody, clientIPs...)
	assert.Equal(tb, 201, status) //nolint:gomnd // Nothing magical about it.

	expectedUsr := new(users.User)
	expectedUsr.ID = userID
	expectedUsr.ReferredBy = referredBy
	expectedUsr.Username = username
	b.AssertCreateUserResponseBody(tb, expectedUsr, body)
}

func (b *TestConnectorsBridge) MustDeleteAllUsers(ctx context.Context, tb testing.TB) {
	tb.Helper()
	userIDs, tokens := b.GetAllTestingAuthorizations()
	wg := new(sync.WaitGroup)
	wg.Add(len(userIDs))
	for ii := range userIDs {
		go func(ix int) {
			defer wg.Done()
			body, status := b.DeleteUser(ctx, tb, userIDs[ix], tokens[ix])
			assert.Empty(tb, body)
			assert.True(tb, status == 204 || status == 200)
		}(ii)
	}
	wg.Wait()
}

//nolint:funlen // A lot of ifs & json, looks better like this.
func (b *TestConnectorsBridge) AssertCreateUserResponseBody(tb testing.TB, expectedUsr *users.User, actualResponseBody string) {
	tb.Helper()
	var referredByKV, phoneNumberKV, emailKV string
	if expectedUsr.ReferredBy != "" {
		referredByKV = fmt.Sprintf(`,"referredBy":%q`, expectedUsr.ReferredBy)
	}
	if expectedUsr.PhoneNumber != "" {
		phoneNumberKV = fmt.Sprintf(`,"phoneNumber":%q`, strings.ReplaceAll(expectedUsr.PhoneNumber, "+", "[+]"))
	}
	if expectedUsr.Email != "" {
		emailKV = fmt.Sprintf(`,"email":%q`, strings.ReplaceAll(strings.ReplaceAll(expectedUsr.Email, ".", "[.]"), "+", "[+]"))
	}
	if expectedUsr.Country == "" {
		expectedUsr.Country = defaultClientIPCountry
	}
	if expectedUsr.City == "" {
		expectedUsr.City = defaultClientIPCity
	} else {
		expectedUsr.City = strings.ReplaceAll(strings.ReplaceAll(expectedUsr.City, "+", "[+]"), ".", "[.]")
	}
	username := strings.ReplaceAll(expectedUsr.Username, ".", "[.]")
	expectedResponseBody := fmt.Sprintf(`{
			"createdAt":%[1]q,
			"updatedAt":%[1]q,
			"id":%[2]q,
			"username":%[3]q
			%[4]v,
			"profilePictureUrl":%[9]q,
			"country":%[5]q,
			"city":%[6]q
			%[7]v
			%[8]v
		}`, timeRegex, expectedUsr.ID, username, phoneNumberKV, expectedUsr.Country, expectedUsr.City, emailKV, referredByKV, defaultProfilePictureURL)
	b.AssertResponseBody(tb, expectedResponseBody, actualResponseBody)
}

func (b *TestConnectorsBridge) GetTestingAuthorizationAt(index int) (userID, token string) {
	allUserIDs, allTokens := b.GetAllTestingAuthorizations()

	return allUserIDs[index], allTokens[index]
}

func (*TestConnectorsBridge) GetAllTestingAuthorizations() (userIDs, tokens []string) {
	allUserIDs := strings.Split(os.Getenv("TESTING_USER_IDS"), ",")
	allTokens := strings.Split(os.Getenv("TESTING_TOKENS"), ",")

	return allUserIDs, allTokens
}

func (*TestConnectorsBridge) AssertResponseBody(tb testing.TB, expectedRespBody, actualRespBody string) {
	tb.Helper()
	assert.Regexp(tb, regexp.MustCompile(strings.ReplaceAll(strings.ReplaceAll(expectedRespBody, "\t", ""), "\n", "")), actualRespBody)
}
