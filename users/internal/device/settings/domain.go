// SPDX-License-Identifier: BUSL-1.1

package devicesettings

import (
	"github.com/framey-io/go-tarantool"
	"github.com/goccy/go-json"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:gomnd // Those are field indexes.
func (ds *DeviceSettings) buildUpdateOps() []tarantool.Op {
	ops := make([]tarantool.Op, 0, 4)
	ops = append(ops, tarantool.Op{Op: "=", Field: 0, Arg: time.Now()})
	if ds.NotificationSettings != nil {
		ops = append(ops, tarantool.Op{Op: "=", Field: 1, Arg: ds.NotificationSettings})
	}
	if ds.Language != nil {
		ops = append(ops, tarantool.Op{Op: "=", Field: 2, Arg: *ds.Language})
	}
	if ds.DisableAllNotifications != nil {
		ops = append(ops, tarantool.Op{Op: "=", Field: 3, Arg: *ds.DisableAllNotifications})
	}

	return ops
}

//nolint:funlen // Barely over the limit. Breaking it would increase the complexity.
func (ds *DeviceSettings) patch(with *DeviceSettings) *DeviceSettings {
	if ds == nil {
		return with
	}
	if with == nil {
		return ds
	}
	ds.requireEqualID(with)
	r := new(DeviceSettings)
	if with.ID.UserID != "" {
		r.ID = with.ID
	} else {
		r.ID = ds.ID
	}
	if with.Language != nil {
		r.Language = with.Language
	} else {
		r.Language = ds.Language
	}
	if with.DisableAllNotifications != nil {
		r.DisableAllNotifications = with.DisableAllNotifications
	} else {
		r.DisableAllNotifications = ds.DisableAllNotifications
	}
	if with.UpdatedAt != nil {
		r.UpdatedAt = with.UpdatedAt
	} else {
		r.UpdatedAt = ds.UpdatedAt
	}
	r.NotificationSettings = ds.NotificationSettings.patch(with.NotificationSettings)

	return r
}

//nolint:gocognit // Wrong.
func (ds *DeviceSettings) requireEqualID(with *DeviceSettings) {
	idIsSetForBoth := with.ID.UserID != "" &&
		ds.ID.UserID != "" &&
		ds.DeviceUniqueID != "" &&
		with.DeviceUniqueID != ""

	if !idIsSetForBoth {
		return
	}
	idsAreEqual := with.ID.UserID == ds.ID.UserID &&
		with.ID.DeviceUniqueID == ds.ID.DeviceUniqueID

	if idsAreEqual {
		return
	}

	log.Panic(errors.Errorf("deviceSettings have different IDs: %#v != %#v", ds.ID, with.ID))
}

func (n *NotificationSettings) DecodeMsgpack(dec *msgpack.Decoder) error {
	v, err := dec.DecodeString()
	if err != nil {
		return errors.Wrap(err, "failed to DecodeString")
	}
	if v == "" || v == "{}" {
		return nil
	}

	return errors.Wrapf(json.Unmarshal([]byte(v), &n), "failed to json.Unmarshall(%v,*NotificationSettings)", v)
}

func (n *NotificationSettings) EncodeMsgpack(enc *msgpack.Encoder) error {
	bytes, err := json.Marshal(n)
	if err != nil {
		return errors.Wrapf(err, "failed to json.Marshal(%#v)", n)
	}
	v := string(bytes)

	return errors.Wrapf(enc.EncodeString(v), "failed to EncodeString(%v)", v)
}

func (n *NotificationSettings) patch(with *NotificationSettings) *NotificationSettings {
	if n == nil {
		return with
	}
	if with == nil {
		return n
	}
	r := new(NotificationSettings)
	*r = make(NotificationSettings, len(AllNotificationDomains))
	for domain, channels := range *n {
		(*r)[domain] = channels.patch((*with)[domain])
	}
	for domain, channels := range *with {
		if _, alreadyPresent := (*r)[domain]; !alreadyPresent {
			(*r)[domain] = defaultChannels().patch(channels)
		}
	}

	return r
}

func (c *NotificationChannels) patch(with *NotificationChannels) *NotificationChannels {
	if c == nil {
		return with
	}
	if with == nil {
		return c
	}
	r := new(NotificationChannels)
	if with.SMS != nil {
		r.SMS = with.SMS
	} else {
		r.SMS = c.SMS
	}
	if with.Push != nil {
		r.Push = with.Push
	} else {
		r.Push = c.Push
	}
	if with.Email != nil {
		r.Email = with.Email
	} else {
		r.Email = c.Email
	}
	if with.InApp != nil {
		r.InApp = with.InApp
	} else {
		r.InApp = c.InApp
	}

	return r
}

func defaultDeviceSettings(id device.ID) *DeviceSettings {
	disableAllNotifications := false
	language := defaultLanguage

	return &DeviceSettings{
		ID:                      id,
		DisableAllNotifications: &disableAllNotifications,
		NotificationSettings:    defaultNotificationSettings(),
		Language:                &language,
	}
}

func defaultNotificationSettings() *NotificationSettings {
	r := make(NotificationSettings, len(AllNotificationDomains))
	for _, notificationDomain := range AllNotificationDomains {
		r[notificationDomain] = defaultChannels()
	}

	return &r
}

func defaultChannels() *NotificationChannels {
	push := true
	email := true
	sms := true
	inApp := true

	return &NotificationChannels{
		Push:  &push,
		Email: &email,
		SMS:   &sms,
		InApp: &inApp,
	}
}
