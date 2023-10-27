// SPDX-License-Identifier: ice License 1.0

package social

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebScrapperInvalidConfig(t *testing.T) {
	sc := newMustWebScraper(string([]byte{0x00}), "")
	require.NotNil(t, sc)

	impl, ok := sc.(*webScraperImpl)
	require.True(t, ok)
	require.NotNil(t, impl)

	require.Panics(t, func() {
		impl.BuildQuery("foo", nil)
	})

	t.Run("EmptyURL", func(t *testing.T) {
		require.Panics(t, func() {
			_ = newMustWebScraper("", "")
		})
	})
}
