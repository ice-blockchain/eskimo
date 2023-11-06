// SPDX-License-Identifier: ice License 1.0

package social

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFacebookBuildURL(t *testing.T) {
	t.Parallel()

	impl := newFacebookVerifier(nil)
	require.NotNil(t, impl)

	url := impl.BuildURL("foo", &Metadata{
		AccessToken: "bar",
	})
	require.Equal(t, "https://graph.facebook.com/foo?access_token=bar", url)
}

func TestFacebookFetchUserHandle(t *testing.T) {
	t.Parallel()

	token := os.Getenv("FACEBOOK_TEST_TOKEN")
	if token == "" {
		t.Skip("SKIP: FACEBOOK_TEST_TOKEN is not set")
	}

	conf := loadConfig()
	require.NotNil(t, conf)

	sc := newMustWebScraper(conf.WebScrapingAPI.URL, conf.WebScrapingAPI.APIKey)
	require.NotNil(t, sc)

	impl := newFacebookVerifier(sc)
	require.NotNil(t, impl)

	url := impl.BuildURL("me", &Metadata{
		AccessToken: token,
	})
	handle, err := impl.FetchUserHandle(context.TODO(), url)
	require.NoError(t, err)
	require.Equal(t, "126358118771158", handle)

	t.Run("BadScrape", func(t *testing.T) {
		impl := newFacebookVerifier(&mockScraper{})
		_, err := impl.FetchUserHandle(context.TODO(), "")
		require.ErrorIs(t, err, ErrScrapeFailed)
	})

	t.Run("BadToken", func(t *testing.T) {
		_, err := impl.FetchUserHandle(context.TODO(), impl.BuildURL("me", &Metadata{
			AccessToken: "foo",
		}))
		require.ErrorIs(t, err, ErrUsernameNotFound)
	})
}

func TestFacebookVerifyUserFeed(t *testing.T) {
	t.Parallel()

	token := os.Getenv("FACEBOOK_TEST_TOKEN")
	if token == "" {
		t.Skip("SKIP: FACEBOOK_TEST_TOKEN is not set")
	}

	conf := loadConfig()
	require.NotNil(t, conf)

	sc := newMustWebScraper(conf.WebScrapingAPI.URL, conf.WebScrapingAPI.APIKey)
	require.NotNil(t, sc)

	impl := newFacebookVerifier(sc)
	require.NotNil(t, impl)

	targetURL := impl.BuildURL("me/feed", &Metadata{AccessToken: token})
	t.Run("Success", func(t *testing.T) {
		require.NoError(t,
			impl.VerifyUserFeed(context.TODO(), targetURL, `Hello @ice_blockchain`),
		)
	})

	t.Run("NoText", func(t *testing.T) {
		require.ErrorIs(t,
			ErrTextNotFound,
			impl.VerifyUserFeed(context.TODO(), targetURL, `Foo`),
		)
	})

	t.Run("BadScrape", func(t *testing.T) {
		err := newFacebookVerifier(&mockScraper{}).VerifyUserFeed(context.TODO(), targetURL, `Foo`)
		require.ErrorIs(t, err, ErrScrapeFailed)
	})
}
