// SPDX-License-Identifier: ice License 1.0

package social

import (
	"context"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/require"
)

func TestTwitterExtractUsernameFromURL(t *testing.T) {
	t.Parallel()

	impl := newTwitterVerifier(nil, nil, nil)
	require.NotNil(t, impl)

	t.Run("OK", func(t *testing.T) {
		username, err := impl.ExtractUsernameFromURL("https://twitter.com/ice_blockchain/status/1712692723336032437")
		require.NoError(t, err)
		require.Equal(t, "ice_blockchain", username)
	})

	t.Run("Invalid", func(t *testing.T) {
		username, err := impl.ExtractUsernameFromURL("foo")
		require.ErrorIs(t, err, ErrUsernameNotFound)
		require.Empty(t, username)
	})
}

type mockScraper struct{}

func (*mockScraper) Scrape(context.Context, string, webScraperOptionsFunc) ([]byte, error) {
	return []byte{}, multierror.Append(ErrScrapeFailed, ErrFetchFailed)
}

func (*mockScraper) Fetch(context.Context, string) ([]byte, error) {
	return []byte{}, multierror.Append(ErrScrapeFailed, ErrFetchFailed)
}

func TestTwitterVerifyFetch(t *testing.T) {
	t.Parallel()

	impl := newTwitterVerifier(new(mockScraper), []string{"twitter.com"}, []string{"US", "MX", "CA"})
	require.NotNil(t, impl)

	t.Run("BadURL", func(t *testing.T) {
		_, err := impl.VerifyPost(context.Background(), &Metadata{PostURL: "foo"})
		require.ErrorIs(t, err, ErrInvalidURL)
	})

	t.Run("FetchFailed", func(t *testing.T) {
		_, err := impl.VerifyPost(context.Background(), &Metadata{PostURL: "https://twitter.com/foo/status/123"})
		require.ErrorIs(t, err, ErrScrapeFailed)
	})
}
