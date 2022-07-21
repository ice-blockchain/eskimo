// SPDX-License-Identifier: BUSL-1.1

package devicemetadata

import (
	"context"
	"net"
	"strings"

	"github.com/framey-io/go-tarantool"
	"github.com/goccy/go-json"
	"github.com/ip2location/ip2location-go"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	appCfg "github.com/ice-blockchain/wintr/config"
	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	"github.com/ice-blockchain/wintr/connectors/storage"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/time"
)

//nolint:gochecknoinits // We're just transforming the embedded files into useful data types.
func init() {
	var countryArray []*country
	//nolint:revive // That's the point.
	log.Panic(json.Unmarshal([]byte(countriesJSON), &countryArray))
	countries = make(map[string]*country)
	for _, c := range countryArray {
		countries[c.IsoCode] = c
	}

	if len(countries) != 249 { //nolint:gomnd // We have 249 countries in ip2location
		log.Panic(errors.Errorf("invalid number of countries %v. Expected 249", len(countries)))
	}
}

func New(db tarantool.Connector, mb messagebroker.Client) DeviceMetadataRepository {
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)
	repo := &repository{db: db, mb: mb}
	if mb != nil {
		var err error
		repo.ip2LocationDB, err = ip2location.OpenDB(cfg.IP2LocationBinaryPath)
		log.Panic(errors.Wrap(err, "unable to open ip2location database"))
	}

	return repo
}

func (r *repository) Close() error {
	if r.ip2LocationDB != nil {
		r.ip2LocationDB.Close()
	}

	return nil
}

func (r *repository) GetDeviceMetadataLocation(ctx context.Context, deviceID device.ID, clientIP net.IP) *DeviceLocation {
	if ctx.Err() != nil {
		log.Error(errors.Wrapf(ctx.Err(), "context error for GetDeviceMetadataLocation for %#v", deviceID))

		return new(DeviceLocation)
	}
	//nolint:godox // .
	//TODO: TBD if we need to use deviceID.DeviceUniqueID and/or deviceID.UserID to find some default/preferred value for the user.

	result, err := r.ip2LocationDB.Get_all(clientIP.String()) //nolint:nosnakecase // External library.
	if err != nil {
		log.Error(errors.Wrapf(err, "unable to get country&city for %#v, %v", deviceID, clientIP.String()))

		return new(DeviceLocation)
	}

	return &DeviceLocation{
		Country: strings.ToUpper(result.Country_short), //nolint:nosnakecase // External library.
		City:    result.City,
	}
}

func (r *repository) GetDeviceMetadata(ctx context.Context, id device.ID) (*DeviceMetadata, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "context failed")
	}
	dm := new(deviceMetadata)
	if err := r.db.GetTyped("DEVICE_METADATA", "pk_unnamed_DEVICE_METADATA_1", id, dm); err != nil {
		return nil, errors.Wrapf(err, "failed to get device metadata by id: %#v", id)
	}
	if dm.ID.UserID == "" {
		return nil, storage.ErrNotFound
	}

	return &dm.DeviceMetadata, nil
}

func (r *repository) ReplaceDeviceMetadata(ctx context.Context, input *DeviceMetadata, clientIP net.IP) (err error) {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	metadata := new(deviceMetadata)
	metadata.UpdatedAt = time.Now()
	metadata.DeviceMetadata = *input
	if metadata.IP2Locationrecord, err = r.ip2LocationDB.Get_all(clientIP.String()); err != nil { //nolint:nosnakecase // External library.
		return errors.Wrapf(err, "failed to get location information based on IP %v to replace device metadata", clientIP.String())
	}
	before, err := r.GetDeviceMetadata(ctx, input.ID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return errors.Wrapf(err, "failed to get current device metadata for %#v", input.ID)
	}
	var result []*deviceMetadata
	if err = r.db.ReplaceTyped("DEVICE_METADATA", metadata, &result); err != nil {
		return errors.Wrapf(err, "failed to replace device's %#v metadata", metadata.ID)
	}
	dm := deviceMetadataSnapshot(before, &result[0].DeviceMetadata)

	return errors.Wrapf(r.sendDeviceMetadataSnapshotMessage(ctx, dm), "failed to send device metadata snapshot message %#v", dm)
}

func deviceMetadataSnapshot(before, after *DeviceMetadata) *DeviceMetadataSnapshot {
	// Because we don't care about the other fields, for now.
	var meta *DeviceMetadata
	if before != nil {
		meta = &DeviceMetadata{
			ID:                    before.ID,
			PushNotificationToken: before.PushNotificationToken,
		}
	}

	return &DeviceMetadataSnapshot{
		DeviceMetadata: &DeviceMetadata{
			ID:                    after.ID,
			PushNotificationToken: after.PushNotificationToken,
		},
		Before: meta,
	}
}

func (r *repository) sendDeviceMetadataSnapshotMessage(ctx context.Context, dm *DeviceMetadataSnapshot) error {
	valueBytes, err := json.Marshal(dm)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal DeviceMetadata %#v", dm)
	}
	msg := &messagebroker.Message{
		Headers: map[string]string{"producer": "eskimo"},
		Key:     dm.UserID + "~" + dm.DeviceUniqueID,
		Topic:   cfg.MessageBroker.Topics[1].Name,
		Value:   valueBytes,
	}
	responder := make(chan error, 1)
	defer close(responder)
	r.mb.SendMessage(ctx, msg, responder)

	return errors.Wrapf(<-responder, "failed to send device metadata message to broker")
}

func (*repository) LookupCountries(keyword Keyword) []Country {
	kw := strings.ToUpper(keyword)
	matchingCountries := make([]Country, 0)
	for countryCode, c := range countries {
		if strings.Contains(countryCode, kw) ||
			strings.Contains(strings.ToUpper(c.Name), kw) {
			matchingCountries = append(matchingCountries, countryCode)
		}
	}

	return matchingCountries
}

func (*repository) IsValid(c Country) bool {
	_, found := countries[strings.ToUpper(c)]

	return found
}
