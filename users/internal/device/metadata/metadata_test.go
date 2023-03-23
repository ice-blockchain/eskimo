// SPDX-License-Identifier: ice License 1.0

package devicemetadata

import (
	"testing"
	stdlibtime "time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	. "github.com/ice-blockchain/wintr/testing"
	"github.com/ice-blockchain/wintr/time"
)

func TestAPIContract(t *testing.T) { //nolint:funlen // You can collapse em.
	t.Parallel()
	if testing.Short() {
		return
	}
	var (
		deviceID = &device.ID{UserID: "a", DeviceUniqueID: "b"}
		datetime = time.New(stdlibtime.UnixMilli(1659737242969))
		location = &DeviceLocation{Country: "a", City: "b"}
		dm       = &DeviceMetadata{
			UpdatedAt:             datetime,
			FirstInstallTime:      datetime,
			LastUpdateTime:        datetime,
			ID:                    *deviceID,
			ReadableVersion:       "a1",
			Fingerprint:           "a2",
			InstanceID:            "a3",
			Hardware:              "a4",
			Product:               "a5",
			Device:                "a6",
			Type:                  "a7",
			Tags:                  "a8",
			DeviceID:              "a9",
			DeviceType:            "a10",
			DeviceName:            "a11",
			Brand:                 "a12",
			Carrier:               "a13",
			Manufacturer:          "a14",
			UserAgent:             "a15",
			SystemName:            "a16",
			SystemVersion:         "a17",
			BaseOS:                "a18",
			BuildID:               "a19",
			Bootloader:            "a20",
			Codename:              "a21",
			InstallerPackageName:  "a22",
			PushNotificationToken: "a23",
			TZ:                    "-07:00",
			APILevel:              1,
			Tablet:                true,
			PinOrFingerprintSet:   true,
			Emulator:              true,
		}
		dmSnapshot = &DeviceMetadataSnapshot{DeviceMetadata: dm, Before: dm}
	)
	AssertSymmetricMarshallingUnmarshalling(t, deviceID, `{
															  "userId": "a",
															  "deviceUniqueId": "b"
														  }`)
	AssertSymmetricMarshallingUnmarshalling(t, location, `{
															  "country": "a",
															  "city": "b"
														  }`)
	AssertSymmetricMarshallingUnmarshalling(t, dm, `{
													  "updatedAt": "2022-08-05T22:07:22.969Z",
													  "firstInstallTime": "2022-08-05T22:07:22.969Z",
													  "lastUpdateTime": "2022-08-05T22:07:22.969Z",
													  "userId": "a",
													  "deviceUniqueId": "b",
													  "readableVersion": "a1",
													  "fingerprint": "a2",
													  "instanceId": "a3",
													  "hardware": "a4",
													  "product": "a5",
													  "device": "a6",
													  "type": "a7",
													  "tags": "a8",
													  "deviceId": "a9",
													  "deviceType": "a10",
													  "deviceName": "a11",
													  "brand": "a12",
													  "carrier": "a13",
													  "manufacturer": "a14",
													  "userAgent": "a15",
													  "systemName": "a16",
													  "systemVersion": "a17",
													  "baseOs": "a18",
													  "buildId": "a19",
													  "bootloader": "a20",
													  "codename": "a21",
													  "installerPackageName": "a22",
													  "pushNotificationToken": "a23",
													  "tz": "-07:00",
													  "apiLevel": 1,
													  "tablet": true,
													  "pinOrFingerprintSet": true,
													  "emulator": true
													}`)
	assert.EqualValues(t, dm, MustUnmarshal[DeviceMetadata](t, `{
													  			  "updatedAt": "2022-08-05T22:07:22.969Z",
																  "firstInstallTime": 1659737242969,
																  "lastUpdateTime": 1659737242969,
																  "userId": "a",
																  "deviceUniqueId": "b",
																  "readableVersion": "a1",
																  "fingerprint": "a2",
																  "instanceId": "a3",
																  "hardware": "a4",
																  "product": "a5",
																  "device": "a6",
																  "type": "a7",
																  "tags": "a8",
																  "deviceId": "a9",
																  "deviceType": "a10",
																  "deviceName": "a11",
																  "brand": "a12",
																  "carrier": "a13",
																  "manufacturer": "a14",
																  "userAgent": "a15",
																  "systemName": "a16",
																  "systemVersion": "a17",
																  "baseOs": "a18",
																  "buildId": "a19",
																  "bootloader": "a20",
																  "codename": "a21",
																  "installerPackageName": "a22",
																  "pushNotificationToken": "a23",
													  			  "tz": "-07:00",
																  "apiLevel": 1,
																  "tablet": true,
																  "pinOrFingerprintSet": true,
																  "emulator": true
																}`))
	AssertSymmetricMarshallingUnmarshalling(t, dmSnapshot, `{
															  "updatedAt": "2022-08-05T22:07:22.969Z",
															  "firstInstallTime": "2022-08-05T22:07:22.969Z",
															  "lastUpdateTime": "2022-08-05T22:07:22.969Z",
															  "userId": "a",
															  "deviceUniqueId": "b",
															  "readableVersion": "a1",
															  "fingerprint": "a2",
															  "instanceId": "a3",
															  "hardware": "a4",
															  "product": "a5",
															  "device": "a6",
															  "type": "a7",
															  "tags": "a8",
															  "deviceId": "a9",
															  "deviceType": "a10",
															  "deviceName": "a11",
															  "brand": "a12",
															  "carrier": "a13",
															  "manufacturer": "a14",
															  "userAgent": "a15",
															  "systemName": "a16",
															  "systemVersion": "a17",
															  "baseOs": "a18",
															  "buildId": "a19",
															  "bootloader": "a20",
															  "codename": "a21",
															  "installerPackageName": "a22",
															  "pushNotificationToken": "a23",
													  		  "tz": "-07:00",
															  "apiLevel": 1,
															  "tablet": true,
															  "pinOrFingerprintSet": true,
															  "emulator": true,
															  "before": {
																"updatedAt": "2022-08-05T22:07:22.969Z",
																"firstInstallTime": "2022-08-05T22:07:22.969Z",
																"lastUpdateTime": "2022-08-05T22:07:22.969Z",
																"userId": "a",
																"deviceUniqueId": "b",
																"readableVersion": "a1",
																"fingerprint": "a2",
																"instanceId": "a3",
																"hardware": "a4",
																"product": "a5",
																"device": "a6",
																"type": "a7",
																"tags": "a8",
																"deviceId": "a9",
																"deviceType": "a10",
																"deviceName": "a11",
																"brand": "a12",
																"carrier": "a13",
																"manufacturer": "a14",
																"userAgent": "a15",
																"systemName": "a16",
																"systemVersion": "a17",
																"baseOs": "a18",
																"buildId": "a19",
																"bootloader": "a20",
																"codename": "a21",
																"installerPackageName": "a22",
																"pushNotificationToken": "a23",
													  		    "tz": "-07:00",
																"apiLevel": 1,
																"tablet": true,
																"pinOrFingerprintSet": true,
																"emulator": true
															  }
															}`)
}
func TestVerifyDeviceAppVersion(t *testing.T) { //nolint:funlen // .
	t.Parallel()
	if testing.Short() {
		return
	}
	repo := repository{cfg: &config{RequiredAppVersion: "v0.0.1"}}
	md := DeviceMetadata{}

	repo.cfg.RequiredAppVersion = "v0.0.1"
	md.ReadableVersion = "v0.0.2"
	require.NoError(t, repo.verifyDeviceAppVersion(&md))

	repo.cfg.RequiredAppVersion = "v0.0.1"
	md.ReadableVersion = "v0.0.1"
	require.NoError(t, repo.verifyDeviceAppVersion(&md))

	repo.cfg.RequiredAppVersion = "v0.0.2"
	md.ReadableVersion = "v0.0.1"
	require.Error(t, repo.verifyDeviceAppVersion(&md))

	repo.cfg.RequiredAppVersion = "v0.0.2"
	md.ReadableVersion = "v0.0.1.2"
	require.Error(t, repo.verifyDeviceAppVersion(&md))

	repo.cfg.RequiredAppVersion = "v0.0.2"
	md.ReadableVersion = "v0.0.2.1"
	require.NoError(t, repo.verifyDeviceAppVersion(&md))

	repo.cfg.RequiredAppVersion = "v0.0.2.0"
	md.ReadableVersion = "v0.0.2"
	require.Error(t, repo.verifyDeviceAppVersion(&md))

	repo.cfg.RequiredAppVersion = "v0.0.2.1"
	md.ReadableVersion = "v0.0.2"
	require.Error(t, repo.verifyDeviceAppVersion(&md))

	repo.cfg.RequiredAppVersion = "v0.0.2.2"
	md.ReadableVersion = "v0.0.2.2"
	require.NoError(t, repo.verifyDeviceAppVersion(&md))

	repo.cfg.RequiredAppVersion = "v0.0.2.1"
	md.ReadableVersion = "v0.0.2.2"
	require.NoError(t, repo.verifyDeviceAppVersion(&md))

	repo.cfg.RequiredAppVersion = "v0.0.2.1288"
	md.ReadableVersion = "v0.0.2.1299"
	require.NoError(t, repo.verifyDeviceAppVersion(&md))

	repo.cfg.RequiredAppVersion = "v0.0.2.1288"
	md.ReadableVersion = "v0.0.2.1287"
	require.Error(t, repo.verifyDeviceAppVersion(&md))
}
