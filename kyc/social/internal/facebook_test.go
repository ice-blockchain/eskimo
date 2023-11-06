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

	impl := newFacebookVerifier(new(nativeScraperImpl), conf.SocialLinks.Facebook.AppID, conf.SocialLinks.Facebook.AppSecret)
	require.NotNil(t, impl)

	meta := &Metadata{AccessToken: token}
	t.Run("Success", func(t *testing.T) {
		require.NoError(t,
			impl.VerifyUserFeed(context.TODO(), meta, `Hello @ice_blockchain`),
		)
	})

	t.Run("NoText", func(t *testing.T) {
		require.ErrorIs(t,
			ErrTextNotFound,
			impl.VerifyUserFeed(context.TODO(), meta, `Foo`),
		)
	})

	t.Run("BadScrape", func(t *testing.T) {
		err := newFacebookVerifier(&mockScraper{}, "1", "2").VerifyUserFeed(context.TODO(), meta, `Foo`)
		require.ErrorIs(t, err, ErrScrapeFailed)
	})
}

func TestFacebookVerifyCtor(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		newFacebookVerifier(nil, "", "")
	})
}
