// SPDX-License-Identifier: ice License 1.0

package social

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTwitterKYC(t *testing.T) {
	t.Parallel()

	const (
		expectedText = `✅ Verifying my account on @ice_blockchain with the nickname: "decanterra"`
		targetURL    = `https://twitter.com/decanterra/status/1717504172810010908`
	)

	conf := loadConfig()
	require.NotNil(t, conf)

	sc := newMustWebScraper(conf.WebScrapingAPI.URL, conf.WebScrapingAPI.APIKey)
	require.NotNil(t, sc)

	verifier := newTwitterVerifier(sc, conf.SocialLinks.Twitter.PostURL, []string{"twitter.com"})
	require.NotNil(t, verifier)

	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute)
	defer cancel()

	username, err := verifier.VerifyPost(ctx, nil, targetURL, expectedText)
	require.NoError(t, err)
	require.Equal(t, "decanterra", username)

	t.Run("EmptyUsername", func(t *testing.T) {
		_, err := verifier.VerifyPost(ctx, nil, "https://twitter.com/foo", expectedText)
		require.ErrorIs(t, err, ErrUsernameNotFound)
	})
}

func TestFacebookKYC(t *testing.T) {
	t.Parallel()

	conf := loadConfig()
	require.NotNil(t, conf)

	sc := newMustWebScraper(conf.WebScrapingAPI.URL, conf.WebScrapingAPI.APIKey)
	require.NotNil(t, sc)

	verifier := newFacebookVerifier(sc)
	require.NotNil(t, verifier)

	t.Skip("SKIP: Facebook KYC is not implemented yet")
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
