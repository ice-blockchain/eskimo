// SPDX-License-Identifier: BUSL-1.1

package users

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/framey-io/go-tarantool"
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
	repeatNum = 20
)

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestUsersProcessor_CreateDeviceSettings_Success(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	repeatWithParallel(t, "CreateDeviceSettings success.", func() {
		ds := generateDeviceSettings(t, getNotificationSettings(randBool(t), randBool(t), randBool(t), randBool(t)), *randBool(t))
		require.NoError(t, usersProcessor.CreateDeviceSettings(ctx, &ds))
		defer fixture.Cleanup(t, getDB(t), &ds)

		res, err := usersRepository.GetDeviceSettings(ctx, ds.ID)
		require.NoError(t, err)
		fixture.CheckDeviceSettings(t, res, &ds)
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

	repeatWithParallel(t, "CreateDeviceSettings failure duplicate.", func() {
		ds := generateDeviceSettings(t, getNotificationSettings(randBool(t), randBool(t), randBool(t), randBool(t)), *randBool(t))
		require.NoError(t, usersProcessor.CreateDeviceSettings(ctx, &ds))
		defer fixture.Cleanup(t, getDB(t), &ds)

		res, err := usersRepository.GetDeviceSettings(ctx, ds.ID)
		require.NoError(t, err)
		fixture.CheckDeviceSettings(t, res, &ds)

		// Duplicate device settings data.
		require.Error(t, usersProcessor.CreateDeviceSettings(ctx, &ds))
	})
}

//nolint:nosnakecase // Our code style allows to use underscores for test functions.
func TestUsersProcessor_CreateDeviceSettings_Success_Nil_Notification_Settings(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	repeatWithParallel(t, "CreateDeviceSettings success nil notification settings.", func() {
		ds := generateDeviceSettings(t, nil, *randBool(t))
		require.NoError(t, usersRepository.CreateDeviceSettings(ctx, &ds))
		defer fixture.Cleanup(t, getDB(t), &ds)

		res, err := usersRepository.GetDeviceSettings(ctx, ds.ID)
		require.NoError(t, err)
		fixture.CheckDeviceSettings(t, res, &ds)
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

	repeatWithParallel(t, "CreateDeviceSettings failure record not exists.", func() {
		ds := generateDeviceSettings(t, getNotificationSettings(randBool(t), randBool(t), randBool(t), randBool(t)), *randBool(t))
		require.Error(t, usersProcessor.ModifyDeviceSettings(ctx, &ds))
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

	repeatWithParallel(t, "ModifyDeviceSettings success.", func() {
		ds := generateDeviceSettings(t, getNotificationSettings(randBool(t), randBool(t), randBool(t), randBool(t)), *randBool(t))
		require.NoError(t, usersProcessor.CreateDeviceSettings(ctx, &ds))

		mds := ds
		mds.NotificationSettings = getNotificationSettings(randBool(t), randBool(t), randBool(t), randBool(t))

		require.NoError(t, usersProcessor.ModifyDeviceSettings(ctx, &mds))
		res, err := usersProcessor.GetDeviceSettings(ctx, device.ID{UserID: ds.UserID, DeviceUniqueID: ds.DeviceUniqueID})
		require.NoError(t, err)
		fixture.CheckDeviceSettings(t, &mds, res)
		verifyDeviceSettingsMessage(ctx, t, &ds, &mds)
		fixture.Cleanup(t, getDB(t), &ds)
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

	repeatWithParallel(t, "ModifyDeviceSettings success nil notification settings.", func() {
		ds := generateDeviceSettings(t, nil, *randBool(t))
		require.NoError(t, usersProcessor.CreateDeviceSettings(ctx, &ds))
		defer fixture.Cleanup(t, getDB(t), &ds)

		res, err := usersProcessor.GetDeviceSettings(ctx, ds.ID)
		require.NoError(t, err)
		fixture.CheckDeviceSettings(t, res, &ds)

		mds := ds
		mds.NotificationSettings = getNotificationSettings(randBool(t), randBool(t), randBool(t), randBool(t))
		modifiedLanguage := "fr"
		mds.Language = &modifiedLanguage
		require.NoError(t, usersProcessor.ModifyDeviceSettings(ctx, &mds))

		res, err = usersProcessor.GetDeviceSettings(ctx, device.ID{UserID: ds.UserID, DeviceUniqueID: ds.DeviceUniqueID})
		require.NoError(t, err)
		verifyDeviceSettingsMessage(ctx, t, res, &mds)
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

	repeatWithParallel(t, "ModifyDeviceSettings failure no record.", func() {
		deviceID := device.ID{UserID: "wrong user id", DeviceUniqueID: "wrong device unique id"}
		_, err := usersRepository.GetDeviceSettings(ctx, deviceID)
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

	repeatWithParallel(t, "GetDeviceSettings success record exists.", func() {
		ds := generateDeviceSettings(t, getNotificationSettings(randBool(t), randBool(t), randBool(t), randBool(t)), *randBool(t))
		require.NoError(t, usersProcessor.CreateDeviceSettings(ctx, &ds))
		defer fixture.Cleanup(t, getDB(t), &ds)

		res, err := usersRepository.GetDeviceSettings(ctx, ds.ID)
		require.NoError(t, err)
		fixture.CheckDeviceSettings(t, res, &ds)
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

	repeatWithParallel(t, "GetDeviceSettings success not full channels.", func() {
		nchPush := randBool(t)
		ds := generateDeviceSettings(t, getNotificationSettings(nchPush, nil, nil, nil), *randBool(t))
		require.NoError(t, usersProcessor.CreateDeviceSettings(ctx, &ds))
		defer fixture.Cleanup(t, getDB(t), &ds)

		res, err := usersRepository.GetDeviceSettings(ctx, ds.ID)
		require.NoError(t, err)

		trueVal := true // All of them are set by default to true if nil.
		ds.NotificationSettings = getNotificationSettings(nchPush, &trueVal, &trueVal, &trueVal)
		fixture.CheckDeviceSettings(t, res, &ds)
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

func verifyDeviceSettingsMessage(ctx context.Context, t *testing.T, before, ds *devicesettings.DeviceSettings) {
	t.Helper()
	valueBytes, err := json.Marshal(DeviceSettingsSnapshot{Before: before, DeviceSettings: ds})
	require.NoError(t, err)
	require.NoError(t, mbConnector.VerifyMessages(ctx, mbfixture.RawMessage{
		Key:   ds.UserID + "~" + ds.DeviceUniqueID,
		Value: string(valueBytes),
		Topic: cfg.MessageBroker.ConsumingTopics[2],
	}))
}

func repeatWithParallel(t *testing.T, funcName string, f func()) {
	t.Helper()
	for i := 0; i < repeatNum; i++ {
		t.Run(funcName, func(t *testing.T) {
			t.Parallel()
			f()
		})
	}
}

func getNotificationSettings(push, email, sms, inApp *bool) *devicesettings.NotificationSettings {
	ns := make(devicesettings.NotificationSettings, len(devicesettings.AllNotificationDomains))
	for _, notificationDomain := range devicesettings.AllNotificationDomains {
		ns[notificationDomain] = &devicesettings.NotificationChannels{
			Push:  push,
			Email: email,
			SMS:   sms,
			InApp: inApp,
		}
	}

	return &ns
}

func generateDeviceSettings(t *testing.T, ns *devicesettings.NotificationSettings, disableAllNotifications bool) devicesettings.DeviceSettings {
	t.Helper()
	lang := randomHex(t, 1)

	return devicesettings.DeviceSettings{
		Language:                &lang,
		DisableAllNotifications: &disableAllNotifications,
		ID:                      device.ID{UserID: generateUserID(t), DeviceUniqueID: uuid.NewString()},
		NotificationSettings:    ns,
	}
}

func generateUserID(t *testing.T) string {
	t.Helper()

	return "did:ethr:" + randomHex(t, 20)
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

func getDB(t *testing.T) tarantool.Connector {
	t.Helper()
	repo, ok := usersRepository.(*repository)
	require.Equal(t, ok, true)

	return repo.db
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
