// SPDX-License-Identifier: BUSL-1.1

package devicesettings

import (
	"context"

	"github.com/framey-io/go-tarantool"
	"github.com/goccy/go-json"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v5"

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

func (r *repository) GetDeviceSettings(ctx context.Context, id device.ID) (*DeviceSettings, error) {
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

func (r *repository) ModifyDeviceSettings(ctx context.Context, ds *DeviceSettings) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	before, err := r.GetDeviceSettings(ctx, ds.ID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return errors.Wrapf(err, "failed to get current device settings for %#v", ds.ID)
		}
		if err = r.insert(ctx, ds); err != nil {
			return errors.Wrapf(err, "failed to insert %#v", ds)
		}
	}
	if before != nil {
		if err = r.update(ctx, ds); err != nil {
			return errors.Wrapf(err, "failed to update device settings %#v", ds)
		}
	}
	snapshot := &DeviceSettingsSnapshot{Before: before, DeviceSettings: ds}

	return errors.Wrapf(r.sendDeviceSettingsSnapshotMessage(ctx, snapshot), "failed to send device settings snapshot message: %#v", snapshot)
}

func (r *repository) insert(ctx context.Context, ds *DeviceSettings) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	if ds.Language == "" {
		ds.Language = "en"
	}
	ds.UpdatedAt = time.Now()
	var resp []*DeviceSettings
	if err := r.db.InsertTyped("DEVICE_SETTINGS", ds, &resp); err != nil {
		return errors.Wrapf(err, "failed to insert %#v", ds)
	}
	*ds = *resp[0]

	return nil
}

func (r *repository) update(ctx context.Context, ds *DeviceSettings) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	var resp []*DeviceSettings
	if err := r.db.UpdateTyped("DEVICE_SETTINGS", "pk_unnamed_DEVICE_SETTINGS_1", ds.ID, ds.buildUpdateOps(), &resp); err != nil {
		tErr := new(tarantool.Error)
		if ok := errors.As(err, tErr); !ok || tErr.Code != tarantool.ER_TUPLE_NOT_FOUND {
			return errors.Wrapf(err, "failed to update device settings %#v", ds)
		}

		return r.ModifyDeviceSettings(ctx, ds)
	}
	*ds = *resp[0]

	return nil
}

//nolint:gomnd // Those are field indexes.
func (ds *DeviceSettings) buildUpdateOps() []tarantool.Op {
	ops := make([]tarantool.Op, 0, 1+1+1)
	ops = append(ops, tarantool.Op{Op: "=", Field: 0, Arg: time.Now()})
	if ds.Language != "" {
		ops = append(ops, tarantool.Op{Op: "=", Field: 4, Arg: ds.Language})
	}
	if len(ds.NotificationSettings) != 0 {
		ops = append(ops, tarantool.Op{Op: "=", Field: 1, Arg: ds.NotificationSettings})
	}

	return ops
}

func (r *repository) sendDeviceSettingsSnapshotMessage(ctx context.Context, ds *DeviceSettingsSnapshot) error {
	valueBytes, err := json.Marshal(ds)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal DeviceSettings %#v", ds)
	}
	m := &messagebroker.Message{
		Headers: map[string]string{"producer": "eskimo"},
		Key:     ds.UserID + "~" + ds.DeviceUniqueID,
		Topic:   cfg.MessageBroker.Topics[2].Name,
		Value:   valueBytes,
	}
	responder := make(chan error, 1)
	defer close(responder)
	r.mb.SendMessage(ctx, m, responder)

	return errors.Wrapf(<-responder, "failed to send device settings message to broker")
}

func (n NotificationSettings) DecodeMsgpack(dec *msgpack.Decoder) error {
	v, err := dec.DecodeString()
	if err != nil {
		return errors.Wrap(err, "failed to DecodeString")
	}
	if v == "" || v == "{}" {
		return nil
	}

	return errors.Wrapf(json.Unmarshal([]byte(v), &n), "failed to json.Unmarshall(%v,*NotificationSettings)", v)
}

func (n NotificationSettings) EncodeMsgpack(enc *msgpack.Encoder) error {
	bytes, err := json.Marshal(n)
	if err != nil {
		return errors.Wrapf(err, "failed to json.Marshal(%#v)", n)
	}
	v := string(bytes)

	return errors.Wrapf(enc.EncodeString(v), "failed to EncodeString(%v)", v)
}
