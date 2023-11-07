// SPDX-License-Identifier: ice License 1.0

package social

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFacebookVerifyUserFeed(t *testing.T) {
	t.Parallel()

	token := os.Getenv("FACEBOOK_TEST_TOKEN")
	if token == "" {
		t.Skip("SKIP: FACEBOOK_TEST_TOKEN is not set")
	}

	conf := loadConfig()
	require.NotNil(t, conf)
	require.NotEmpty(t, conf.SocialLinks.Facebook.PostURL)

	impl := newFacebookVerifier(new(dataFetcherImpl), conf.SocialLinks.Facebook.PostURL, conf.SocialLinks.Facebook.AppID, conf.SocialLinks.Facebook.AppSecret)
	require.NotNil(t, impl)

	const userID = `126358118771158`

	t.Run("Success", func(t *testing.T) {
		meta := &Metadata{AccessToken: token, ExpectedPostText: `Verifying nickname for #ice.`}
		require.NoError(t,
			impl.VerifyUserFeed(context.TODO(), meta, userID),
		)
	})

	t.Run("NoText", func(t *testing.T) {
		meta := &Metadata{AccessToken: token, ExpectedPostText: `Foo`}
		require.ErrorIs(t,
			ErrTextNotFound,
			impl.VerifyUserFeed(context.TODO(), meta, userID),
		)
	})

	t.Run("Norepost", func(t *testing.T) {
		meta := &Metadata{AccessToken: token, ExpectedPostText: `Hello @ice_blockchain`}
		require.ErrorIs(t,
			impl.VerifyUserFeed(context.TODO(), meta, userID),
			ErrPostNotFound,
		)
	})

	t.Run("BadScrape", func(t *testing.T) {
		err := newFacebookVerifier(new(mockScraper), "1", "2", "3").VerifyUserFeed(context.TODO(), &Metadata{}, `1`)
		require.ErrorIs(t, err, ErrScrapeFailed)
	})
}

func TestFacebookVerifyCtor(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		newFacebookVerifier(nil, "", "", "")
	})
}
