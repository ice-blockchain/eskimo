// SPDX-License-Identifier: ice License 1.0

package fixture

import (
	"context"
	"testing"
	stdlibtime "time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	devicemetadata "github.com/ice-blockchain/eskimo/users/internal/device/metadata"
	"github.com/ice-blockchain/go-tarantool-client"
	appCfg "github.com/ice-blockchain/wintr/config"
	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	messagebrokerfixture "github.com/ice-blockchain/wintr/connectors/message_broker/fixture"
	"github.com/ice-blockchain/wintr/time"
)

func AllExistingDeviceMetadata(ctx context.Context, tb testing.TB, db tarantool.Connector, userID string) (resp []*devicemetadata.DeviceMetadata) {
	tb.Helper()
	require.NoError(tb, ctx.Err())
	const all = 1000
	partialKey := tarantool.StringKey{S: userID}
	require.NoError(tb, db.SelectTyped("DEVICE_METADATA", "pk_unnamed_DEVICE_METADATA_1", 0, all, tarantool.IterEq, partialKey, &resp))

	return resp
}

func VerifyNoDeviceMetadataSnapshotMessages(
	ctx context.Context, tb testing.TB, mbConnector messagebrokerfixture.TestConnector, tpe DeviceMetadataSnapshotMessageType, deviceIDs ...device.ID,
) {
	tb.Helper()
	require.NoError(tb, ctx.Err())
	var cfg struct {
		messagebroker.Config `mapstructure:",squash"` //nolint:tagliatelle // Nope.
	}
	appCfg.MustLoadFromKey("users", &cfg)
	messages := make([]messagebrokerfixture.RawMessage, 0, len(deviceIDs))
	for ix := range deviceIDs {
		var valueRegex string
		switch tpe {
		case ANY:
			valueRegex = "^.*$"
		case CREATE:
			valueRegex = `^{((?!,"before":).)*}$`
		case UPDATE:
			valueRegex = `^{.+,"before":{.+}$`
		case DELETE:
			valueRegex = `^{"before":{.+}$`
		}
		messages = append(messages, messagebrokerfixture.RawMessage{
			Key:   deviceIDs[ix].UserID + "~~~" + deviceIDs[ix].DeviceUniqueID,
			Value: valueRegex,
			Topic: cfg.MessageBroker.Topics[1].Name,
		})
	}
	windowedCtx, cancelWindowed := context.WithTimeout(ctx, 2*stdlibtime.Second) //nolint:gomnd // .
	defer cancelWindowed()
	assert.NoError(tb, mbConnector.VerifyNoMessages(windowedCtx, messages...))
}

func CompletelyRandomizeDeviceMetadata(userID string) *devicemetadata.DeviceMetadata { //nolint:funlen // Alot of fields.
	return &devicemetadata.DeviceMetadata{
		UpdatedAt:        time.Now(),
		FirstInstallTime: time.Now(),
		LastUpdateTime:   time.Now(),
		ID: device.ID{
			UserID:         userID,
			DeviceUniqueID: uuid.NewString(),
		},
		ReadableVersion:       "v0.0.1",
		Fingerprint:           uuid.NewString(),
		InstanceID:            uuid.NewString(),
		Hardware:              uuid.NewString(),
		Product:               uuid.NewString(),
		Device:                uuid.NewString(),
		Type:                  uuid.NewString(),
		Tags:                  uuid.NewString(),
		DeviceID:              uuid.NewString(),
		DeviceType:            uuid.NewString(),
		DeviceName:            uuid.NewString(),
		Brand:                 uuid.NewString(),
		Carrier:               uuid.NewString(),
		Manufacturer:          uuid.NewString(),
		UserAgent:             uuid.NewString(),
		SystemName:            uuid.NewString(),
		SystemVersion:         uuid.NewString(),
		BaseOS:                uuid.NewString(),
		BuildID:               uuid.NewString(),
		Bootloader:            uuid.NewString(),
		Codename:              uuid.NewString(),
		InstallerPackageName:  uuid.NewString(),
		PushNotificationToken: uuid.NewString(),
		APILevel:              31, //nolint:gomnd // .
		Tablet:                false,
		PinOrFingerprintSet:   false,
		Emulator:              false,
	}
}

const (
	ANY DeviceMetadataSnapshotMessageType = iota
	CREATE
	UPDATE
	DELETE
)

type (
	DeviceMetadataSnapshotMessageType byte
)
