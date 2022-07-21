// SPDX-License-Identifier: BUSL-1.1

package devicesettings

import (
	"context"

	"github.com/framey-io/go-tarantool"
	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	appCfg "github.com/ice-blockchain/wintr/config"
	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/time"
)

func New(db tarantool.Connector, mb messagebroker.Client) DeviceSettingsRepository {
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)

	return &repository{db: db, mb: mb}
}

func (r *repository) getDeviceSettings(ctx context.Context, id device.ID) (*DeviceSettings, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "context failed")
	}
	ds := new(DeviceSettings)
	if err := r.db.GetTyped("DEVICE_SETTINGS", "pk_unnamed_DEVICE_SETTINGS_1", id, ds); err != nil {
		return nil, errors.Wrapf(err, "failed to get device settings for id: %#v", id)
	}
	if ds.ID.UserID == "" {
		return nil, storage.ErrNotFound
	}

	return ds, nil
}

//nolint:revive // There's no other way to rename them.
func (r *repository) GetDeviceSettings(ctx context.Context, id device.ID) (*DeviceSettings, error) {
	settings, err := r.getDeviceSettings(ctx, id)
	if err == nil {
		settings = defaultDeviceSettings(id).patch(settings)
	}

	return settings, err
}

func (r *repository) CreateDeviceSettings(ctx context.Context, settings *DeviceSettings) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}

	return errors.Wrapf(r.insert(ctx, settings), "failed to insert %#v", settings)
}

func (r *repository) ModifyDeviceSettings(ctx context.Context, ds *DeviceSettings) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	before, err := r.getDeviceSettings(ctx, ds.ID)
	if err != nil {
		return errors.Wrapf(err, "failed to get current device settings for %#v", ds.ID)
	}
	if err = r.update(ctx, before, ds); err != nil {
		return errors.Wrapf(err, "failed to update device settings %#v", ds)
	}
	snapshot := &DeviceSettingsSnapshot{Before: before, DeviceSettings: ds}

	return errors.Wrapf(r.sendDeviceSettingsSnapshotMessage(ctx, snapshot), "failed to send device settings snapshot message: %#v", snapshot)
}

func (r *repository) insert(ctx context.Context, ds *DeviceSettings) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	ds.UpdatedAt = time.Now()
	*ds = *(defaultDeviceSettings(ds.ID).patch(ds))
	var resp []*DeviceSettings
	if err := r.db.InsertTyped("DEVICE_SETTINGS", ds, &resp); err != nil {
		tErr := new(tarantool.Error)
		if ok := errors.As(err, tErr); ok && tErr.Code == tarantool.ER_TUPLE_FOUND { //nolint:nosnakecase // External library.
			err = storage.ErrDuplicate
		}

		return errors.Wrapf(err, "failed to insert %#v", ds)
	}
	*ds = *resp[0]

	return nil
}

func (r *repository) update(ctx context.Context, before, ds *DeviceSettings) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	*ds = *(defaultDeviceSettings(ds.ID).patch(before).patch(ds))
	var resp []*DeviceSettings
	if err := r.db.UpdateTyped("DEVICE_SETTINGS", "pk_unnamed_DEVICE_SETTINGS_1", ds.ID, ds.buildUpdateOps(), &resp); err != nil {
		tErr := new(tarantool.Error)
		if ok := errors.As(err, tErr); ok && tErr.Code == tarantool.ER_TUPLE_NOT_FOUND { //nolint:nosnakecase // External library.
			err = storage.ErrNotFound
		}

		return errors.Wrapf(err, "failed to update device settings %#v", ds)
	}
	*ds = *resp[0]

	return nil
}

func (r *repository) sendDeviceSettingsSnapshotMessage(ctx context.Context, ds *DeviceSettingsSnapshot) error {
	valueBytes, err := json.Marshal(ds)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal DeviceSettings %#v", ds)
	}
	msg := &messagebroker.Message{
		Headers: map[string]string{"producer": "eskimo"},
		Key:     ds.UserID + "~" + ds.DeviceUniqueID,
		Topic:   cfg.MessageBroker.Topics[2].Name,
		Value:   valueBytes,
	}
	responder := make(chan error, 1)
	defer close(responder)
	r.mb.SendMessage(ctx, msg, responder)

	return errors.Wrapf(<-responder, "failed to send device settings message to broker")
}
