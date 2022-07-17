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
	res := new(DeviceSettings)
	if with.ID.UserID != "" {
		res.ID = with.ID
	} else {
		res.ID = ds.ID
	}
	if with.Language != nil {
		res.Language = with.Language
	} else {
		res.Language = ds.Language
	}
	if with.DisableAllNotifications != nil {
		res.DisableAllNotifications = with.DisableAllNotifications
	} else {
		res.DisableAllNotifications = ds.DisableAllNotifications
	}
	if with.UpdatedAt != nil {
		res.UpdatedAt = with.UpdatedAt
	} else {
		res.UpdatedAt = ds.UpdatedAt
	}
	res.NotificationSettings = ds.NotificationSettings.patch(with.NotificationSettings)

	return res
}

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
	val, err := dec.DecodeString()
	if err != nil {
		return errors.Wrap(err, "failed to DecodeString")
	}
	if val == "" || val == "{}" {
		return nil
	}

	return errors.Wrapf(json.Unmarshal([]byte(val), &n), "failed to json.Unmarshall(%v,*NotificationSettings)", val)
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
	res := new(NotificationSettings)
	*res = make(NotificationSettings, len(AllNotificationDomains))
	for domain, channels := range *n {
		(*res)[domain] = channels.patch((*with)[domain])
	}
	for domain, channels := range *with {
		if _, alreadyPresent := (*res)[domain]; !alreadyPresent {
			(*res)[domain] = defaultChannels().patch(channels)
		}
	}

	return res
}

func (c *NotificationChannels) patch(with *NotificationChannels) *NotificationChannels {
	if c == nil {
		return with
	}
	if with == nil {
		return c
	}
	res := new(NotificationChannels)
	if with.SMS != nil {
		res.SMS = with.SMS
	} else {
		res.SMS = c.SMS
	}
	if with.Push != nil {
		res.Push = with.Push
	} else {
		res.Push = c.Push
	}
	if with.Email != nil {
		res.Email = with.Email
	} else {
		res.Email = c.Email
	}
	if with.InApp != nil {
		res.InApp = with.InApp
	} else {
		res.InApp = c.InApp
	}

	return res
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
