// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	devicesettings "github.com/ice-blockchain/eskimo/users/internal/device/settings"
	"github.com/ice-blockchain/eskimo/users/internal/device/settings/fixture"
	mbfixture "github.com/ice-blockchain/wintr/connectors/message_broker/fixture"
)

const (
	repeatNum = 3
)

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestUsersProcessor_CreateDeviceSettings_Success(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	repeatScenarioInParallel(t, func() {
		givenDeviceSettings := shouldGenerateRandomDeviceSettings(t, shouldGenerateRandomNotificationSettings(t))

		require.NoError(t, usersProcessor.CreateDeviceSettings(ctx, givenDeviceSettings))

		thenExpectDeviceSettingsInDB(ctx, t, givenDeviceSettings)
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestUsersProcessor_CreateDeviceSettings_Failure_Duplicate(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	repeatScenarioInParallel(t, func() {
		givenDeviceSettings := shouldGenerateRandomDeviceSettings(t, shouldGenerateRandomNotificationSettings(t))
		require.NoError(t, usersProcessor.CreateDeviceSettings(ctx, givenDeviceSettings))

		thenExpectDeviceSettingsInDB(ctx, t, givenDeviceSettings)

		// Duplicate device settings data.
		require.Error(t, usersProcessor.CreateDeviceSettings(ctx, givenDeviceSettings))
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestUsersProcessor_CreateDeviceSettings_Success_Given_Nil_Notification_Settings(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	repeatScenarioInParallel(t, func() {
		givenDeviceSettings := shouldGenerateRandomDeviceSettings(t, nil)

		require.NoError(t, usersRepository.CreateDeviceSettings(ctx, givenDeviceSettings))

		thenExpectDeviceSettingsInDB(ctx, t, givenDeviceSettings)
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestUsersProcessor_ModifyDeviceSettings_Failure_Record_Not_Exists(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	repeatScenarioInParallel(t, func() {
		givenDeviceSettings := shouldGenerateRandomDeviceSettings(t, shouldGenerateRandomNotificationSettings(t))

		require.Error(t, usersProcessor.ModifyDeviceSettings(ctx, givenDeviceSettings))
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestUsersProcessor_ModifyDeviceSettings_Success(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	repeatScenarioInParallel(t, func() {
		givenDeviceSettings := shouldGenerateRandomDeviceSettings(t, shouldGenerateRandomNotificationSettings(t))
		require.NoError(t, usersProcessor.CreateDeviceSettings(ctx, givenDeviceSettings))

		modifiedDeviceSettings := givenDeviceSettings
		modifiedDeviceSettings.NotificationSettings = shouldGenerateRandomNotificationSettings(t)

		require.NoError(t, usersProcessor.ModifyDeviceSettings(ctx, modifiedDeviceSettings))

		thenExpectDeviceSettingsInDB(ctx, t, modifiedDeviceSettings)
		mustVerifyDeviceSettingsMessage(ctx, t, givenDeviceSettings, modifiedDeviceSettings)
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestUsersProcessor_ModifyDeviceSettings_Success_Nil_Notification_Settings(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	repeatScenarioInParallel(t, func() {
		givenDeviceSettings := shouldGenerateRandomDeviceSettings(t, nil)
		require.NoError(t, usersProcessor.CreateDeviceSettings(ctx, givenDeviceSettings))

		thenExpectDeviceSettingsInDB(ctx, t, givenDeviceSettings)

		modifiedDeviceSettings := givenDeviceSettings
		modifiedDeviceSettings.NotificationSettings = shouldGenerateRandomNotificationSettings(t)
		modifiedDeviceSettings.Language = shouldGenerateRandomLanguage(t)

		require.NoError(t, usersProcessor.ModifyDeviceSettings(ctx, modifiedDeviceSettings))

		thenExpectDeviceSettingsInDB(ctx, t, modifiedDeviceSettings)
		mustVerifyDeviceSettingsMessage(ctx, t, givenDeviceSettings, modifiedDeviceSettings)
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestUsersProcessor_ModifyDeviceSettings_Failure_NoRecord(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	repeatScenarioInParallel(t, func() {
		givenDeviceID := device.ID{UserID: randomHex(t, 10), DeviceUniqueID: uuid.NewString()}

		_, err := usersRepository.GetDeviceSettings(ctx, givenDeviceID)
		require.Error(t, err)
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestUsersRepository_GetDeviceSettings_Success(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	repeatScenarioInParallel(t, func() {
		givenDeviceSettings := shouldGenerateRandomDeviceSettings(t, shouldGenerateRandomNotificationSettings(t))
		require.NoError(t, usersProcessor.CreateDeviceSettings(ctx, givenDeviceSettings))

		thenExpectDeviceSettingsInDB(ctx, t, givenDeviceSettings)
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestUsersRepository_GetDeviceSettings_Success_NotFullChannels(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	repeatScenarioInParallel(t, func() {
		givenPushValue := randBool(t)
		givenDeviceSettings := shouldGenerateRandomDeviceSettings(t, shouldGenerateNotificationSettings(t, givenPushValue, nil, nil, nil))
		require.NoError(t, usersProcessor.CreateDeviceSettings(ctx, givenDeviceSettings))

		thenExpectDeviceSettingsInDB(ctx, t, givenDeviceSettings)

		trueVal := true // All of channels are set by default to true if they are nil.
		givenDeviceSettings.NotificationSettings = shouldGenerateNotificationSettings(t, givenPushValue, &trueVal, &trueVal, &trueVal)
		thenExpectDeviceSettingsInDB(ctx, t, givenDeviceSettings)
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestUsersProcessor_ModifyDeviceSettings_Failure_ContextTimeout(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Nanosecond)
	defer cancel()

	time.Sleep(60 * time.Nanosecond)
	require.Error(t, usersProcessor.ModifyDeviceSettings(ctx, &devicesettings.DeviceSettings{}))
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestUsersProcessor_CreateDeviceSettings_Failure_ContextTimeout(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Nanosecond)
	defer cancel()

	time.Sleep(60 * time.Nanosecond)
	require.Error(t, usersProcessor.CreateDeviceSettings(ctx, &devicesettings.DeviceSettings{}))
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestUsersRepository_CreateDeviceSettings_Failure_ContextTimeout(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Nanosecond)
	defer cancel()

	time.Sleep(60 * time.Nanosecond)
	_, err := usersRepository.GetDeviceSettings(ctx, device.ID{UserID: "doesn't matter", DeviceUniqueID: "doesn't matter"})
	require.Error(t, err)
}

func mustVerifyDeviceSettingsMessage(ctx context.Context, t *testing.T, before, ds *devicesettings.DeviceSettings) {
	t.Helper()
	valueBytes, err := json.Marshal(DeviceSettingsSnapshot{Before: before, DeviceSettings: ds})
	require.NoError(t, err)
	require.NoError(t, mbConnector.VerifyMessages(ctx, mbfixture.RawMessage{
		Key:   ds.UserID + "~" + ds.DeviceUniqueID,
		Value: string(valueBytes),
		Topic: cfg.MessageBroker.ConsumingTopics[2],
	}))
}

func repeatScenarioInParallel(t *testing.T, f func()) {
	t.Helper()
	for i := 0; i < repeatNum; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			f()
		})
	}
}

func shouldGenerateRandomNotificationSettings(t *testing.T) *devicesettings.NotificationSettings {
	t.Helper()

	return shouldGenerateNotificationSettings(t, randBool(t), randBool(t), randBool(t), randBool(t))
}

func shouldGenerateNotificationSettings(t *testing.T, push, email, sms, inApp *bool) *devicesettings.NotificationSettings {
	t.Helper()
	notificationSettings := make(devicesettings.NotificationSettings, len(devicesettings.AllNotificationDomains))
	for _, notificationDomain := range devicesettings.AllNotificationDomains {
		notificationSettings[notificationDomain] = &devicesettings.NotificationChannels{
			Push:  push,
			Email: email,
			SMS:   sms,
			InApp: inApp,
		}
	}

	return &notificationSettings
}

func shouldGenerateRandomDeviceSettings(t *testing.T, ns *devicesettings.NotificationSettings) *devicesettings.DeviceSettings {
	t.Helper()
	lang := randomHex(t, 1)

	return &devicesettings.DeviceSettings{
		Language:                &lang,
		DisableAllNotifications: randBool(t),
		ID:                      device.ID{UserID: shouldGenerateUserID(t), DeviceUniqueID: uuid.NewString()},
		NotificationSettings:    ns,
	}
}

func shouldGenerateRandomLanguage(t *testing.T) *string {
	t.Helper()
	lang := randomHex(t, 2)

	return &lang
}

func shouldGenerateUserID(t *testing.T) string {
	t.Helper()

	return "did:ethr:" + randomHex(t, 20)
}

func thenExpectDeviceSettingsInDB(ctx context.Context, t *testing.T, givenDeviceSettings *DeviceSettings) {
	t.Helper()
	res, err := usersRepository.GetDeviceSettings(ctx, givenDeviceSettings.ID)
	require.NoError(t, err)
	fixture.ExpectDeviceSettings(t, res, givenDeviceSettings)
}

func randomHex(t *testing.T, num int) string {
	t.Helper()
	//nolint:makezero // Because otherwise we have an empty value.
	bytes := make([]byte, num)
	n, err := rand.Read(bytes)
	assert.Equal(t, n, len(bytes))
	require.NoError(t, err)

	return hex.EncodeToString(bytes)
}

func randBool(t *testing.T) *bool {
	t.Helper()
	num, err := rand.Int(rand.Reader, big.NewInt(2))
	require.NoError(t, err)
	var val bool
	if num.Int64() == 1 {
		val = true
	} else {
		val = false
	}

	return &val
}
