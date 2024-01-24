// SPDX-License-Identifier: ice License 1.0

package social

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTwitterKYC(t *testing.T) {
	t.Parallel()

	const (
		expectedText    = `✅ Verifying my account on @ice_blockchain with the nickname: "foo"`
		expectedPostURL = `/ice_blockchain/status/1712692723336032437`
		targetURL       = `https://twitter.com/JohnDoe1495747/status/1734542700005732621`
	)

	conf := loadConfig()
	require.NotNil(t, conf)

	sc := newMustWebScraper(conf.WebScrapingAPI.URL, conf.WebScrapingAPI.APIKey)
	require.NotNil(t, sc)

	verifier := newTwitterVerifier(sc, []string{"twitter.com"}, []string{"US", "MX", "CA"})
	require.NotNil(t, verifier)

	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute)
	defer cancel()

	username, err := verifier.VerifyPost(ctx, &Metadata{PostURL: targetURL, ExpectedPostText: expectedText, ExpectedPostURL: expectedPostURL})
	require.NoError(t, err)
	require.Equal(t, "JohnDoe1495747", username)

	t.Run("EmptyUsername", func(t *testing.T) {
		_, err := verifier.VerifyPost(ctx, &Metadata{PostURL: "https://twitter.com/foo", ExpectedPostText: expectedText})
		require.ErrorIs(t, err, ErrUsernameNotFound)
	})
}

func TestTwitterKYCNoRepost(t *testing.T) {
	t.Parallel()

	const (
		expectedText = `✅ Verifying my account on @ice_blockchain with the nickname: "john"`
		targetURL    = `https://twitter.com/JohnDoe1495747/status/1750103621184700443`
	)

	conf := loadConfig()
	require.NotNil(t, conf)

	sc := newMustWebScraper(conf.WebScrapingAPI.URL, conf.WebScrapingAPI.APIKey)
	require.NotNil(t, sc)

	verifier := newTwitterVerifier(sc, []string{"twitter.com"}, []string{"US", "MX", "CA"})
	require.NotNil(t, verifier)

	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute)
	defer cancel()

	username, err := verifier.VerifyPost(ctx, &Metadata{PostURL: targetURL, ExpectedPostText: expectedText})
	require.NoError(t, err)
	require.Equal(t, "JohnDoe1495747", username)
}

func TestTwitterPrivate(t *testing.T) {
	t.Parallel()

	conf := loadConfig()
	require.NotNil(t, conf)

	sc := newMustWebScraper(conf.WebScrapingAPI.URL, conf.WebScrapingAPI.APIKey)
	require.NotNil(t, sc)

	verifier := newTwitterVerifier(sc, []string{"twitter.com"}, []string{"US", "MX", "CA"})
	require.NotNil(t, verifier)

	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute)
	defer cancel()

	_, err := verifier.VerifyPost(ctx, &Metadata{PostURL: `https://twitter.com/root/status/1748008059103039495`, ExpectedPostText: "foo", ExpectedPostURL: "bar"})
	require.ErrorIs(t, err, ErrTweetPrivate)
}

func TestFacebookKYC(t *testing.T) {
	t.Parallel()

	token := os.Getenv("FACEBOOK_TEST_TOKEN")
	if token == "" {
		t.Skip("SKIP: FACEBOOK_TEST_TOKEN is not set")
	}

	conf := loadConfig()
	require.NotNil(t, conf)

	verifier := New(StrategyFacebook)
	require.NotNil(t, verifier)

	username, err := verifier.VerifyPost(context.TODO(),
		&Metadata{
			AccessToken:      token,
			ExpectedPostText: `Verifying nickname for #ice.`,
		})
	require.NoError(t, err)
	require.Equal(t, "126358118771158", username)
}

func TestStrategyNew(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		New("foo")
	})

	impl := New(StrategyTwitter)
	require.NotNil(t, impl)

	impl = New(StrategyFacebook)
	require.NotNil(t, impl)
}
