// SPDX-License-Identifier: BUSL-1.1

package fixture

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/framey-io/go-tarantool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	settings "github.com/ice-blockchain/eskimo/users/internal/device/settings"
)

func CheckDeviceSettings(t *testing.T, current, expected *settings.DeviceSettings) {
	t.Helper()
	assert.Regexp(t, regexp.MustCompile(timeRegex), current.UpdatedAt)
	assert.Equal(t, current.UserID, expected.UserID)
	assert.Equal(t, current.DeviceUniqueID, expected.DeviceUniqueID)
	assert.Equal(t, *current.DisableAllNotifications, *expected.DisableAllNotifications)
	assert.Equal(t, *current.Language, *expected.Language)
	assert.Equal(t, *current.NotificationSettings, *expected.NotificationSettings)
}

func Cleanup(t *testing.T, db tarantool.Connector, ds *settings.DeviceSettings) {
	t.Helper()
	rk := device.ID{
		UserID:         ds.UserID,
		DeviceUniqueID: ds.DeviceUniqueID,
	}
	require.NoError(t, db.DeleteTyped(deviceSettingsTable(), deviceSettingsPKIndex(), rk, &[]*settings.DeviceSettings{}))
	checkIfDeviceSettingsDeleted(t, db, ds)
}

func checkIfDeviceSettingsDeleted(t *testing.T, db tarantool.Connector, ds *settings.DeviceSettings) {
	t.Helper()
	rk := device.ID{UserID: ds.UserID, DeviceUniqueID: ds.DeviceUniqueID}
	var res []*settings.DeviceSettings
	err := db.GetTyped(deviceSettingsTable(), deviceSettingsPKIndex(), rk, &res)
	require.NoError(t, err)
	require.Empty(t, res)
}

func deviceSettingsTable() string {
	return "DEVICE_SETTINGS"
}

func deviceSettingsPKIndex() string {
	return fmt.Sprintf("pk_unnamed_%v_1", deviceSettingsTable())
}
