// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	devicemetadata "github.com/ice-blockchain/eskimo/users/internal/device/metadata"
	"github.com/ice-blockchain/go-tarantool-client"
)

func (c *CountryStatistics) bindExisting(ctx context.Context, tb testing.TB, country devicemetadata.Country) *CountryStatistics {
	tb.Helper()
	require.NoError(tb, ctx.Err())
	require.NoError(tb, dbConnector.GetTyped("USERS_PER_COUNTRY", "pk_unnamed_USERS_PER_COUNTRY_1", tarantool.StringKey{S: country}, c))

	return c
}

func assertUsersPerCountry(ctx context.Context, tb testing.TB, country devicemetadata.Country, expected uint64) {
	tb.Helper()
	require.NoError(tb, ctx.Err())
	windowedCtx, windowedCancel := context.WithTimeout(ctx, 2*time.Second)
	defer windowedCancel()

	var actual uint64
	for windowedCtx.Err() == nil {
		if actual = new(CountryStatistics).bindExisting(ctx, tb, country).UserCount; actual == expected {
			return
		}
	}

	assert.Fail(tb, "unexpected users per country", "expected %v, actual %v", fmt.Sprint(expected), fmt.Sprint(actual))
}
