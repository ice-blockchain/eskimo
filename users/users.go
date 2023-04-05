// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync"
	stdlibtime "time"

	"github.com/cenkalti/backoff/v4"
	"github.com/goccy/go-json"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v5"

	devicemetadata "github.com/ice-blockchain/eskimo/users/internal/device/metadata"
	"github.com/ice-blockchain/go-tarantool-client"
	"github.com/ice-blockchain/wintr/analytics/tracking"
	appCfg "github.com/ice-blockchain/wintr/config"
	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	"github.com/ice-blockchain/wintr/connectors/storage"
	storagev2 "github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/multimedia/picture"
	"github.com/ice-blockchain/wintr/time"
)

func New(ctx context.Context, cancel context.CancelFunc) Repository {
	var cfg config
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)

	db := storage.MustConnect(ctx, cancel, ddl, applicationYamlKey)
	dbV2 := storagev2.MustConnect(ctx, ddlV2, applicationYamlKey)

	return &repository{
		cfg:                      &cfg,
		shutdown:                 db.Close,
		db:                       db,
		dbV2:                     dbV2,
		DeviceMetadataRepository: devicemetadata.New(db, nil),
		pictureClient:            picture.New(applicationYamlKey),
	}
}

func StartProcessor(ctx context.Context, cancel context.CancelFunc) Processor {
	var cfg config
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)

	var mbConsumer messagebroker.Client
	db := storage.MustConnect(context.Background(), func() { //nolint:contextcheck // It's intended. Cuz we want to close everything gracefully.
		if mbConsumer != nil {
			log.Error(errors.Wrap(mbConsumer.Close(), "failed to close mbConsumer due to db premature cancellation"))
		}
		cancel()
	}, ddl, applicationYamlKey)
	dbV2 := storagev2.MustConnect(ctx, ddlV2, applicationYamlKey)
	mbProducer := messagebroker.MustConnect(ctx, applicationYamlKey)
	prc := &processor{repository: &repository{
		cfg:                      &cfg,
		db:                       db,
		dbV2:                     dbV2,
		mb:                       mbProducer,
		DeviceMetadataRepository: devicemetadata.New(db, mbProducer),
		pictureClient:            picture.New(applicationYamlKey, defaultProfilePictureNameRegex),
		trackingClient:           tracking.New(applicationYamlKey),
	}}
	mbConsumer = messagebroker.MustConnectAndStartConsuming(context.Background(), cancel, applicationYamlKey, //nolint:contextcheck // It's intended.
		&userSnapshotSource{processor: prc},
		&miningSessionSource{processor: prc},
		&userPingSource{processor: prc},
	)
	prc.shutdown = closeAll(mbConsumer, prc.mb, prc.db, prc.DeviceMetadataRepository.Close)

	return prc
}

func (r *repository) Close() error {
	return errors.Wrap(r.shutdown(), "closing users repository failed")
}

func closeAll(mbConsumer, mbProducer messagebroker.Client, db tarantool.Connector, otherClosers ...func() error) func() error {
	return func() error {
		err1 := errors.Wrap(mbConsumer.Close(), "closing message broker consumer connection failed")
		err2 := errors.Wrap(db.Close(), "closing db connection failed")
		err3 := errors.Wrap(mbProducer.Close(), "closing message broker producer connection failed")
		errs := make([]error, 0, 1+1+1+len(otherClosers))
		errs = append(errs, err1, err2, err3)
		for _, closeOther := range otherClosers {
			if err := closeOther(); err != nil {
				errs = append(errs, err)
			}
		}

		return errors.Wrap(multierror.Append(nil, errs...).ErrorOrNil(), "failed to close resources")
	}
}

func (p *processor) CheckHealth(ctx context.Context) error {
	if _, err := p.db.Ping(); err != nil {
		return errors.Wrap(err, "[health-check] failed to ping DB")
	}
	type ts struct {
		TS *time.Time `json:"ts"`
	}
	now := ts{TS: time.Now()}
	bytes, err := json.MarshalContext(ctx, now)
	if err != nil {
		return errors.Wrapf(err, "[health-check] failed to marshal %#v", now)
	}
	responder := make(chan error, 1)
	p.mb.SendMessage(ctx, &messagebroker.Message{
		Headers: map[string]string{"producer": "eskimo"},
		Key:     p.cfg.MessageBroker.Topics[0].Name,
		Topic:   p.cfg.MessageBroker.Topics[0].Name,
		Value:   bytes,
	}, responder)

	return errors.Wrapf(<-responder, "[health-check] failed to send health check message to broker")
}

func retry(ctx context.Context, op func() error) error {
	//nolint:wrapcheck // No need, its just a proxy.
	return backoff.RetryNotify(
		op,
		//nolint:gomnd // Because those are static configs.
		backoff.WithContext(&backoff.ExponentialBackOff{
			InitialInterval:     100 * stdlibtime.Millisecond,
			RandomizationFactor: 0.5,
			Multiplier:          2.5,
			MaxInterval:         stdlibtime.Second,
			MaxElapsedTime:      25 * stdlibtime.Second,
			Stop:                backoff.Stop,
			Clock:               backoff.SystemClock,
		}, ctx),
		func(e error, next stdlibtime.Duration) {
			log.Error(errors.Wrapf(e, "call failed. retrying in %v... ", next))
		})
}

func randomBetween(left, right uint64) uint64 {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(right)-int64(left)))
	log.Panic(errors.Wrap(err, "crypto random generator failed"))

	return n.Uint64() + left
}

func requestingUserID(ctx context.Context) (requestingUserID string) {
	requestingUserID, _ = ctx.Value(requestingUserIDCtxValueKey).(string) //nolint:errcheck // Not needed.

	return
}

func lastUpdatedAt(ctx context.Context) *time.Time {
	checksum, ok := ctx.Value(checksumCtxValueKey).(string)
	if !ok || checksum == "" {
		return nil
	}

	nanos, err := strconv.Atoi(checksum)
	if err != nil {
		log.Error(errors.Wrapf(err, "checksum %v is not numeric", checksum))

		return nil
	}

	return time.New(stdlibtime.Unix(0, int64(nanos)))
}

func ContextWithChecksum(ctx context.Context, checksum string) context.Context {
	if true || checksum == "" { //nolint:revive // TODO:: to be decided if this feature is still needed.
		return ctx
	}

	return context.WithValue(ctx, checksumCtxValueKey, checksum) //nolint:revive,staticcheck // Not an issue.
}

func (n *NotExpired) DecodeMsgpack(dec *msgpack.Decoder) error {
	v := new(time.Time)
	if err := v.DecodeMsgpack(dec); err != nil {
		return errors.Wrap(err, "failed to decode time based struct for NotExpired")
	}
	*n = NotExpired(time.Now().Before(*v.Time))

	return nil
}

func (n *NotExpired) Scan(src any) error {
	date, ok := src.(stdlibtime.Time)
	if ok {
		date = date.UTC()
		*n = NotExpired(time.Now().Before(date))

		return nil
	}

	return errors.Errorf("unexpected type for src:%#v(%T)", src, src)
}

func (e *Enum[T]) DecodeMsgpack(decoder *msgpack.Decoder) error {
	enumStr, err := decoder.DecodeString()
	if err != nil {
		return errors.Wrap(err, "failed to decode string")
	}
	if enumStr == "" {
		*e = nil

		return nil
	}
	eDeref := *e
	parts := strings.Split(enumStr, ",")
	if len(eDeref) == 0 {
		eDeref = make(Enum[T], 0, len(parts))
	}
	for _, elem := range parts {
		eDeref = append(eDeref, T(elem))
	}
	*e = eDeref

	return nil
}

func (e *Enum[T]) Scan(src any) error {
	enumStr, isStr := src.(string)
	if isStr {
		if enumStr == "" {
			*e = nil

			return nil
		}
		eDeref := *e
		parts := strings.Split(enumStr, ",")
		if len(eDeref) == 0 {
			eDeref = make(Enum[T], 0, len(parts))
		}
		for _, elem := range parts {
			eDeref = append(eDeref, T(elem))
		}
		*e = eDeref

		return nil
	}

	return errors.Errorf("unexpected type for src:%#v(%T)", src, src)
}

func (e *Enum[T]) EncodeMsgpack(encoder *msgpack.Encoder) error {
	if e == nil || len(*e) == 0 {
		return errors.Wrap(encoder.EncodeNil(), "failed to encode nil")
	}
	eDeref := *e
	enum := make([]string, 0, len(eDeref))
	for _, elem := range eDeref {
		enum = append(enum, string(elem))
	}

	return errors.Wrap(encoder.EncodeString(strings.Join(enum, ",")), "failed to encode string")
}

func (j *JSON) Scan(src any) error {
	val, isStr := src.(string)
	if isStr {
		if val == "" {
			return nil
		}
		if val == "{}" {
			*j = make(JSON, 0)
		}

		return errors.Wrapf(json.UnmarshalContext(context.Background(), []byte(val), j), "failed to json.Unmarshall(%v,*JSON)", val)
	}

	return errors.Errorf("unexpected type for src:%#v(%T)", src, src)
}

func (j *JSON) DecodeMsgpack(dec *msgpack.Decoder) error {
	val, err := dec.DecodeString()
	if err != nil {
		return errors.Wrap(err, "failed to DecodeString")
	}
	if val == "" {
		return nil
	}
	if val == "{}" {
		*j = make(JSON, 0)
	}

	return errors.Wrapf(json.UnmarshalContext(context.Background(), []byte(val), j), "failed to json.Unmarshall(%v,*JSON)", val)
}

func (j *JSON) EncodeMsgpack(enc *msgpack.Encoder) error {
	if j == nil || len(*j) == 0 {
		return errors.Wrap(enc.EncodeNil(), "failed to encode nil")
	}
	bytes, err := json.MarshalContext(context.Background(), *j)
	if err != nil {
		return errors.Wrapf(err, "failed to json.Marshal(%#v)", *j)
	}
	v := string(bytes)

	return errors.Wrapf(enc.EncodeString(v), "failed to EncodeString(%v)", v)
}

func (u *User) Checksum() string {
	if u.UpdatedAt == nil {
		return ""
	}

	return fmt.Sprint(u.UpdatedAt.UnixNano())
}

func (u *User) sanitizeForUI() {
	u.RandomReferredBy = nil
	u.PhoneNumberHash = ""
	u.HashCode = 0
	if u.Username == u.ID {
		u.Username = ""
	}
	if u.ReferredBy == u.ID {
		u.ReferredBy = ""
	}
}

func (r *repository) sanitizeUser(usr *User) *User {
	usr.LastMiningStartedAt = nil
	usr.LastMiningEndedAt = nil
	usr.LastPingCooldownEndedAt = nil
	if usr.BlockchainAccountAddress == usr.ID {
		usr.BlockchainAccountAddress = ""
	}
	if usr.MiningBlockchainAccountAddress == usr.ID {
		usr.MiningBlockchainAccountAddress = ""
	}
	if usr.PhoneNumber == usr.ID {
		usr.PhoneNumber = ""
	}
	if usr.PhoneNumberHash == usr.ID {
		usr.PhoneNumberHash = ""
	}
	if usr.Email == usr.ID {
		usr.Email = ""
	}
	if usr.Username == usr.ID {
		usr.Username = ""
	}
	if usr.ReferredBy == usr.ID {
		usr.ReferredBy = ""
	}
	usr.ProfilePictureURL = r.pictureClient.DownloadURL(usr.ProfilePictureURL)

	return usr
}

func (c *config) globalAggregationIntervalChildDateFormat() string {
	const hoursInADay = 24
	switch c.GlobalAggregationInterval.Child { //nolint:exhaustive // We don't care about the others.
	case stdlibtime.Minute:
		return minuteFormat
	case stdlibtime.Hour:
		return hourFormat
	case hoursInADay * stdlibtime.Hour:
		return dayFormat
	default:
		log.Panic(fmt.Sprintf("invalid interval: %v", c.GlobalAggregationInterval.Child))

		return ""
	}
}

func (c *config) globalAggregationIntervalParentDateFormat() string {
	const hoursInADay = 24
	switch c.GlobalAggregationInterval.Parent { //nolint:exhaustive // We don't care about the others.
	case stdlibtime.Minute:
		return minuteFormat
	case stdlibtime.Hour:
		return hourFormat
	case hoursInADay * stdlibtime.Hour:
		return dayFormat
	default:
		log.Panic(fmt.Sprintf("invalid interval: %v", c.GlobalAggregationInterval.Parent))

		return ""
	}
}

func (c *config) nanosSinceGlobalAggregationIntervalParentZeroValue(now *time.Time) stdlibtime.Duration {
	const hoursInADay = 24
	switch c.GlobalAggregationInterval.Parent { //nolint:exhaustive // We don't care about the others.
	case stdlibtime.Minute:
		return stdlibtime.Duration(now.Nanosecond()) +
			stdlibtime.Duration(now.Second())*stdlibtime.Second
	case stdlibtime.Hour:
		return stdlibtime.Duration(now.Nanosecond()) +
			stdlibtime.Duration(now.Second())*stdlibtime.Second +
			stdlibtime.Duration(now.Minute())*stdlibtime.Minute
	case hoursInADay * stdlibtime.Hour:
		return stdlibtime.Duration(now.Nanosecond()) +
			stdlibtime.Duration(now.Second())*stdlibtime.Second +
			stdlibtime.Duration(now.Minute())*stdlibtime.Minute +
			stdlibtime.Duration(now.Hour())*stdlibtime.Hour
	default:
		log.Panic(fmt.Sprintf("invalid interval: %v", c.GlobalAggregationInterval.Parent))

		return 0
	}
}

func RandomizeHiddenProfileElements() *Enum[HiddenProfileElement] {
	maxHPECount := uint64(len(HiddenProfileElements)) + 1
	left := randomBetween(0, maxHPECount)
	right := randomBetween(0, maxHPECount)
	right = uint64(math.Max(float64(left), float64(right)))
	shuffled := HiddenProfileElements[left:right]

	return &shuffled
}

func RandomDefaultProfilePictureName() string {
	return fmt.Sprintf(defaultProfilePictureName, randomBetween(1, totalNoOfDefaultProfilePictures+1))
}

func mergePointerToArrayField[T comparable, ArrT interface{ ~[]T }](oldData, newData *ArrT) *ArrT {
	if newData != nil {
		newDataRef := *newData
		cpy := append(make(ArrT, 0, len(newDataRef)), newDataRef...)

		return &cpy
	}

	return oldData
}

func mergePointerToMapField[K comparable, V any, MapKV interface{ ~map[K]V }](oldData, newData *MapKV) *MapKV {
	if newData != nil {
		newDataRef := *newData
		cpy := make(MapKV, len(newDataRef))
		for k, v := range newDataRef {
			cpy[k] = v
		}

		return &cpy
	}

	return oldData
}

func mergePointerField[T comparable](oldData, newData *T) *T {
	if newData != nil {
		cpy := new(T)
		*cpy = *newData

		return cpy
	}

	return oldData
}

func mergeTimeField(oldData, newData *time.Time) *time.Time {
	if newData != nil {
		return time.New(stdlibtime.Unix(0, newData.UnixNano()))
	}

	return oldData
}

func mergeStringField(oldData, newData string) string {
	if newData != "" {
		return newData
	}

	return oldData
}

func sendMessagesConcurrently[M any](ctx context.Context, sendMessage func(context.Context, *M) error, messages []*M) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline")
	}
	if len(messages) == 0 {
		return nil
	}
	wg := new(sync.WaitGroup)
	wg.Add(len(messages))
	errChan := make(chan error, len(messages))
	for i := range messages {
		go func(ix int) {
			defer wg.Done()
			errChan <- errors.Wrapf(sendMessage(ctx, messages[ix]), "failed to sendMessage:%#v", messages[ix])
		}(i)
	}
	wg.Wait()
	close(errChan)
	errs := make([]error, 0, len(messages))
	for err := range errChan {
		errs = append(errs, err)
	}

	return errors.Wrap(multierror.Append(nil, errs...).ErrorOrNil(), "at least one message sends failed")
}
