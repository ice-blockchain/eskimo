// SPDX-License-Identifier: ice License 1.0

package devicemetadata

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/goccy/go-json"
	"github.com/hashicorp/go-multierror"
	"github.com/ip2location/ip2location-go/v9"
	"github.com/pkg/errors"
	"golang.org/x/mod/semver"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	"github.com/ice-blockchain/go-tarantool-client"
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
	log.Panic(json.UnmarshalContext(context.Background(), []byte(countriesJSON), &countryArray))
	countries = make(map[string]*country)
	for _, c := range countryArray {
		countries[c.IsoCode] = c
	}

	if len(countries) != 249 { //nolint:gomnd // We have 249 countries in ip2location
		log.Panic(errors.Errorf("invalid number of countries %v. Expected 249", len(countries)))
	}
}

func New(db tarantool.Connector, mb messagebroker.Client) DeviceMetadataRepository {
	var cfg config
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)
	repo := &repository{db: db, mb: mb, cfg: &cfg}
	if mb != nil {
		var err error
		repo.ip2LocationDB, err = ip2location.OpenDB(cfg.IP2LocationBinaryPath)
		log.Panic(errors.Wrap(err, "unable to open ip2location database"))
	}

	return repo
}

func (r *repository) DeleteAllDeviceMetadata(ctx context.Context, userID string) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline ")
	}
	sql := `SELECT * FROM device_metadata WHERE user_id = :user_id`
	params := map[string]any{"user_id": userID}
	var res []*DeviceMetadata
	if err := r.db.PrepareExecuteTyped(sql, params, &res); err != nil {
		return errors.Wrapf(err, "failed to select all device metadata for userID:%v", userID)
	}
	if len(res) == 0 {
		return nil
	}
	wg := new(sync.WaitGroup)
	wg.Add(len(res))
	errChan := make(chan error, len(res))
	for ix := range res {
		go func(iix int) {
			defer wg.Done()
			errChan <- errors.Wrapf(r.deleteDeviceMetadata(ctx, res[iix]), "failed to deleteDeviceMetadata for %#v", res[iix])
		}(ix)
	}
	wg.Wait()
	close(errChan)
	errs := make([]error, 0, len(res))
	for err := range errChan {
		errs = append(errs, err)
	}

	return multierror.Append(nil, errs...).ErrorOrNil() //nolint:wrapcheck // Not needed.
}

func (r *repository) deleteDeviceMetadata(ctx context.Context, dm *DeviceMetadata) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	if err := r.db.DeleteTyped("DEVICE_METADATA", "pk_unnamed_DEVICE_METADATA_1", &dm.ID, &[]*DeviceMetadata{}); err != nil {
		return errors.Wrapf(err, "failed to delete device_metadata for id:%#v", &dm.ID)
	}
	snapshot := deviceMetadataSnapshot(dm, nil)
	if err := r.sendDeviceMetadataSnapshotMessage(ctx, snapshot); err != nil {
		return errors.Wrapf(err, "failed to sendDeviceMetadataSnapshotMessage for %#v", snapshot)
	}
	if err := r.sendTombstonedDeviceMetadataMessage(ctx, &dm.ID); err != nil {
		return errors.Wrapf(err, "failed to sendTombstonedDeviceMetadataMessage for %#v", &dm.ID)
	}

	return nil
}

func (r *repository) Close() error {
	if r.ip2LocationDB != nil {
		r.ip2LocationDB.Close()
	}

	return nil
}

func (r *repository) GetDeviceMetadataLocation(ctx context.Context, deviceID *device.ID, clientIP net.IP) *DeviceLocation {
	if ctx.Err() != nil {
		log.Error(errors.Wrapf(ctx.Err(), "context error for GetDeviceMetadataLocation for %#v", deviceID))

		return new(DeviceLocation)
	}
	//nolint:godox // .
	// TODO: TBD if we need to use deviceID.DeviceUniqueID and/or deviceID.UserID to find some default/preferred value for the user.

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

func (r *repository) GetDeviceMetadata(ctx context.Context, id *device.ID) (*DeviceMetadata, error) {
	if ctx.Err() != nil {
		return nil, errors.Wrap(ctx.Err(), "context failed")
	}
	dm := new(DeviceMetadata)
	if err := r.db.GetTyped("DEVICE_METADATA", "pk_unnamed_DEVICE_METADATA_1", id, dm); err != nil {
		return nil, errors.Wrapf(err, "failed to get device metadata by id: %#v", id)
	}
	if dm.ID.UserID == "" {
		return nil, storage.ErrNotFound
	}

	return dm, nil
}

func (r *repository) ReplaceDeviceMetadata(ctx context.Context, input *DeviceMetadata, clientIP net.IP) (err error) { //nolint:funlen // Big rollback logic.
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context failed")
	}
	if vErr := r.verifyDeviceAppVersion(input); vErr != nil {
		return vErr
	}
	if input.UserID == "" || input.UserID == "-" {
		return nil
	}
	input.UpdatedAt = time.Now()
	var ip2locationRecord ip2location.IP2Locationrecord
	if ip2locationRecord, err = r.ip2LocationDB.Get_all(clientIP.String()); err != nil { //nolint:nosnakecase // External library.
		return errors.Wrapf(err, "failed to get location information based on IP %v to replace device metadata", clientIP.String())
	}
	(&input.ip2LocationRecord).convertIP2Location(&ip2locationRecord)
	before, err := r.GetDeviceMetadata(ctx, &input.ID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return errors.Wrapf(err, "failed to get current device metadata for %#v", input.ID)
	}
	if err = storage.CheckSQLDMLErr(r.db.PrepareExecute(input.replaceSQL())); err != nil {
		return errors.Wrapf(err, "failed to replace device's %#v metadata", input.ID)
	}
	dm := deviceMetadataSnapshot(before, input)
	if err = r.sendDeviceMetadataSnapshotMessage(ctx, dm); err != nil {
		var revertErr error
		if before == nil {
			revertErr = errors.Wrapf(r.db.DeleteTyped("DEVICE_METADATA", "pk_unnamed_DEVICE_METADATA_1", &input.ID, &[]*DeviceMetadata{}),
				"failed to delete device metadata due to rollback for %#v", input)
		} else {
			revertErr = errors.Wrapf(storage.CheckSQLDMLErr(r.db.PrepareExecute(before.replaceSQL())),
				"failed to replace to before, due to a rollback for %#v", input)
		}

		return multierror.Append(errors.Wrapf(err, "failed to send device metadata snapshot message %#v", dm), revertErr).ErrorOrNil() //nolint:wrapcheck // .
	}

	return nil
}

func (r *repository) verifyDeviceAppVersion(metadata *DeviceMetadata) error {
	readableParts := strings.Split(metadata.ReadableVersion, ".")
	if len(readableParts) < 1+1+1 {
		return errors.Wrapf(ErrInvalidAppVersion, "invalid version %v", metadata.ReadableVersion)
	}
	version := strings.ReplaceAll(fmt.Sprintf("v%v.%v.%v", readableParts[0], readableParts[1], readableParts[2]), "vv", "v")
	if semver.Compare(version, r.cfg.RequiredAppVersion) < 0 {
		return errors.Wrapf(ErrOutdatedAppVersion,
			"mobile app version %v is older than the required one %v, please update", metadata.ReadableVersion, r.cfg.RequiredAppVersion)
	}

	return errors.Wrapf(r.verifyDeviceAppNanosVersion(readableParts),
		"mobile app version %v is older than the required one %v, please update", metadata.ReadableVersion, r.cfg.RequiredAppVersion)
}

func (r *repository) verifyDeviceAppNanosVersion(readableParts []string) error { //nolint:gocognit // .
	requiredParts := strings.Split(r.cfg.RequiredAppVersion, ".")
	if len(requiredParts) > 1+1+1 && len(readableParts) == 1+1+1 {
		return errors.Wrapf(ErrOutdatedAppVersion,
			"mobile app version doesn't contain nanos that is required %v, please update", r.cfg.RequiredAppVersion)
	}
	if len(requiredParts) > 1+1+1 && len(readableParts) > 1+1+1 {
		readableNano, err := strconv.Atoi(readableParts[3])
		log.Panic(err) //nolint:revive // Wrong, it's reachable.
		requiredNano, err := strconv.Atoi(requiredParts[3])
		log.Panic(err)

		if readableNano != 0 && readableNano != 1 && readableNano < requiredNano {
			return errors.Wrapf(ErrOutdatedAppVersion,
				"mobile app nanos version %v is older than the required one %v, please update", readableNano, requiredNano)
		}
	}

	return nil
}

//nolint:funlen // A lot of fields here.
func (dm *DeviceMetadata) replaceSQL() (string, map[string]any) {
	sql := `REPLACE INTO DEVICE_METADATA (
				country_short,
				country_long,
				region,
				city,
				isp,
				latitude,
				longitude,
				domain,
				zipcode,
				timezone,
				net_speed,
				idd_code,
				area_code,
				weather_station_code,
				weather_station_name,
				mcc,
				mnc,
				mobile_brand,
				elevation,
				usage_type,
				updated_at,
				first_install_time,
				last_update_time,
				user_id,
				device_unique_id,
				readable_version,
				fingerprint,
				instance_id,
				hardware,
				product,
				device,
				type,
				tags,
				device_id,
				device_type,
				device_name,
				brand,
				carrier,
				manufacturer,
				user_agent,
				system_name,
				system_version,
				base_os,
				build_id,
				bootloader,
				codename,
				installer_package_name,
				push_notification_token,
				device_timezone,
				api_level,
				tablet,
				pin_or_fingerprint_set,
				emulator
			) VALUES (
				:country_short,
				:country_long,
				:region,
				:city,
				:isp,
				:latitude,
				:longitude,
				:domain,
				:zipcode,
				:timezone,
				:net_speed,
				:idd_code,
				:area_code,
				:weather_station_code,
				:weather_station_name,
				:mcc,
				:mnc,
				:mobile_brand,
				:elevation,
				:usage_type,
				:updated_at,
				:first_install_time,
				:last_update_time,
				:user_id,
				:device_unique_id,
				:readable_version,
				:fingerprint,
				:instance_id,
				:hardware,
				:product,
				:device,
				:type,
				:tags,
				:device_id,
				:device_type,
				:device_name,
				:brand,
				:carrier,
				:manufacturer,
				:user_agent,
				:system_name,
				:system_version,
				:base_os,
				:build_id,
				:bootloader,
				:codename,
				:installer_package_name,
				:push_notification_token,
				:device_timezone,
				:api_level,
				:tablet,
				:pin_or_fingerprint_set,
				:emulator
			)`
	params := map[string]any{
		"country_short":           dm.CountryShort,
		"country_long":            dm.CountryLong,
		"region":                  dm.Region,
		"city":                    dm.City,
		"isp":                     dm.Isp,
		"latitude":                dm.Latitude,
		"longitude":               dm.Longitude,
		"domain":                  dm.Domain,
		"zipcode":                 dm.Zipcode,
		"timezone":                dm.Timezone,
		"net_speed":               dm.Netspeed,
		"idd_code":                dm.Iddcode,
		"area_code":               dm.Areacode,
		"weather_station_code":    dm.Weatherstationcode,
		"weather_station_name":    dm.Weatherstationname,
		"mcc":                     dm.Mcc,
		"mnc":                     dm.Mnc,
		"mobile_brand":            dm.Mobilebrand,
		"elevation":               dm.Elevation,
		"usage_type":              dm.Usagetype,
		"updated_at":              dm.UpdatedAt,
		"first_install_time":      dm.FirstInstallTime,
		"last_update_time":        dm.LastUpdateTime,
		"user_id":                 dm.UserID,
		"device_unique_id":        dm.DeviceUniqueID,
		"readable_version":        dm.ReadableVersion,
		"fingerprint":             dm.Fingerprint,
		"instance_id":             dm.InstanceID,
		"hardware":                dm.Hardware,
		"product":                 dm.Product,
		"device":                  dm.Device,
		"type":                    dm.Type,
		"tags":                    dm.Tags,
		"device_id":               dm.DeviceID,
		"device_type":             dm.DeviceType,
		"device_name":             dm.DeviceName,
		"brand":                   dm.Brand,
		"carrier":                 dm.Carrier,
		"manufacturer":            dm.Manufacturer,
		"user_agent":              dm.UserAgent,
		"system_name":             dm.SystemName,
		"system_version":          dm.SystemVersion,
		"base_os":                 dm.BaseOS,
		"build_id":                dm.BuildID,
		"bootloader":              dm.Bootloader,
		"codename":                dm.Codename,
		"installer_package_name":  dm.InstallerPackageName,
		"push_notification_token": dm.PushNotificationToken,
		"device_timezone":         dm.TZ,
		"api_level":               dm.APILevel,
		"tablet":                  dm.Tablet,
		"pin_or_fingerprint_set":  dm.PinOrFingerprintSet,
		"emulator":                dm.Emulator,
	}

	return sql, params
}

func (rec *ip2LocationRecord) convertIP2Location(ip *ip2location.IP2Locationrecord) {
	*rec = ip2LocationRecord{
		CountryShort:       ip.Country_short, //nolint:nosnakecase // 3rd party library.
		CountryLong:        ip.Country_long,  //nolint:nosnakecase // 3rd party library.
		Region:             ip.Region,
		City:               ip.City,
		Isp:                ip.Isp,
		Domain:             ip.Domain,
		Zipcode:            ip.Zipcode,
		Timezone:           ip.Timezone,
		Netspeed:           ip.Netspeed,
		Iddcode:            ip.Iddcode,
		Areacode:           ip.Areacode,
		Weatherstationcode: ip.Weatherstationcode,
		Weatherstationname: ip.Weatherstationname,
		Mcc:                ip.Mcc,
		Mnc:                ip.Mnc,
		Mobilebrand:        ip.Mobilebrand,
		Usagetype:          ip.Usagetype,
		Elevation:          float64(ip.Elevation),
		Latitude:           float64(ip.Latitude),
		Longitude:          float64(ip.Longitude),
	}
}

func deviceMetadataSnapshot(before, after *DeviceMetadata) *DeviceMetadataSnapshot {
	var before2, after2 *DeviceMetadata
	if before != nil {
		before2 = &DeviceMetadata{
			ID:                    before.ID,
			PushNotificationToken: before.PushNotificationToken,
			TZ:                    before.ip2LocationRecord.Timezone,
		}
		if before.TZ != "" {
			before2.TZ = before.TZ
		}
	}
	if after != nil {
		after2 = &DeviceMetadata{
			ID:                    after.ID,
			PushNotificationToken: after.PushNotificationToken,
			TZ:                    after.ip2LocationRecord.Timezone,
		}
		if after.TZ != "" {
			after2.TZ = after.TZ
		}
	}

	return &DeviceMetadataSnapshot{DeviceMetadata: after2, Before: before2}
}

func (r *repository) sendTombstonedDeviceMetadataMessage(ctx context.Context, did *device.ID) error {
	msg := &messagebroker.Message{
		Headers: map[string]string{"producer": "eskimo"},
		Key:     did.UserID + "~~~" + did.DeviceUniqueID,
		Topic:   r.cfg.MessageBroker.Topics[2].Name,
	}
	responder := make(chan error, 1)
	defer close(responder)
	r.mb.SendMessage(ctx, msg, responder)

	return errors.Wrapf(<-responder, "failed to send tombstoned device metadata message to broker")
}

func (r *repository) sendDeviceMetadataSnapshotMessage(ctx context.Context, dm *DeviceMetadataSnapshot) error {
	valueBytes, err := json.MarshalContext(ctx, dm)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal DeviceMetadata %#v", dm)
	}
	var did *device.ID
	if dm.DeviceMetadata != nil {
		did = &dm.DeviceMetadata.ID
	} else {
		did = &dm.Before.ID
	}
	msg := &messagebroker.Message{
		Headers: map[string]string{"producer": "eskimo"},
		Key:     did.UserID + "~~~" + did.DeviceUniqueID,
		Topic:   r.cfg.MessageBroker.Topics[2].Name,
		Value:   valueBytes,
	}
	responder := make(chan error, 1)
	defer close(responder)
	r.mb.SendMessage(ctx, msg, responder)

	return errors.Wrapf(<-responder, "failed to send device metadata snapshot message to broker")
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

func RandomCountry() Country {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(countries))))
	log.Panic(errors.Wrap(err, "crypto random generator failed")) //nolint:revive // Should never happen.

	currentIdx := uint64(0)
	desiredIdx := n.Uint64()
	for k := range countries {
		if currentIdx == desiredIdx {
			return k
		}
		currentIdx++
	}

	return ""
}
