// SPDX-License-Identifier: ice License 1.0

package social

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasRootDomainAndHTTPS(t *testing.T) {
	t.Parallel()

	require.True(t, hasRootDomainAndHTTPS("https://www.facebook.com/story", "facebook.com"))
	require.True(t, hasRootDomainAndHTTPS("https://facebook.com/story", "facebook.com"))
	require.True(t, hasRootDomainAndHTTPS("https://m.facebook.com/foo", "facebook.com"))

	require.False(t, hasRootDomainAndHTTPS("http://m.facebook.com/foo", "facebook.com"))
	require.False(t, hasRootDomainAndHTTPS("https://m.facebook.com/foo", "twitter.com"))
	require.False(t, hasRootDomainAndHTTPS("https://mysuperfacebook.com/foo", "facebook.com"))
	require.False(t, hasRootDomainAndHTTPS("https://mysuperfacebook.com/foo", "facebook"))

	require.False(t, hasRootDomainAndHTTPS("\011", "facebook.com"))
}
