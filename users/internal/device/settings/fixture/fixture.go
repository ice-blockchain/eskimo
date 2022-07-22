// SPDX-License-Identifier: BUSL-1.1

package fixture

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	settings "github.com/ice-blockchain/eskimo/users/internal/device/settings"
)

func ExpectDeviceSettings(t *testing.T, current, expected *settings.DeviceSettings) {
	t.Helper()
	assert.Regexp(t, regexp.MustCompile(timeRegex), current.UpdatedAt)
	assert.Equal(t, current.UserID, expected.UserID)
	assert.Equal(t, current.DeviceUniqueID, expected.DeviceUniqueID)
	assert.Equal(t, *current.DisableAllNotifications, *expected.DisableAllNotifications)
	assert.Equal(t, *current.Language, *expected.Language)
	assert.Equal(t, *current.NotificationSettings, *expected.NotificationSettings)
}
