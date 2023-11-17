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

	"github.com/goccy/go-json"
	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pkg/errors"

	devicemetadata "github.com/ice-blockchain/eskimo/users/internal/device/metadata"
	"github.com/ice-blockchain/wintr/analytics/tracking"
	appcfg "github.com/ice-blockchain/wintr/config"
	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	storage "github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/multimedia/picture"
	"github.com/ice-blockchain/wintr/time"
)

func New(ctx context.Context, _ context.CancelFunc) Repository {
	var cfg config
	appcfg.MustLoadFromKey(applicationYamlKey, &cfg)

	db := storage.MustConnect(ctx, ddl, applicationYamlKey)

	return &repository{
		cfg:                      &cfg,
		shutdown:                 db.Close,
		db:                       db,
		DeviceMetadataRepository: devicemetadata.New(db, nil),
		pictureClient:            picture.New(applicationYamlKey),
	}
}

func StartProcessor(ctx context.Context, cancel context.CancelFunc) Processor {
	var cfg config
	appcfg.MustLoadFromKey(applicationYamlKey, &cfg)

	var mbConsumer messagebroker.Client
	db := storage.MustConnect(ctx, ddl, applicationYamlKey)
	mbProducer := messagebroker.MustConnect(ctx, applicationYamlKey)
	prc := &processor{repository: &repository{
		cfg:                      &cfg,
		db:                       db,
		mb:                       mbProducer,
		DeviceMetadataRepository: devicemetadata.New(db, mbProducer),
		pictureClient:            picture.New(applicationYamlKey, defaultProfilePictureNameRegex),
	}}
	if !cfg.DisableConsumer {
		prc.trackingClient = tracking.New(applicationYamlKey)
		mbConsumer = messagebroker.MustConnectAndStartConsuming(context.Background(), cancel, applicationYamlKey, //nolint:contextcheck // It's intended.
			&userSnapshotSource{processor: prc},
			&miningSessionSource{processor: prc},
			&userPingSource{processor: prc},
		)
		go prc.startOldProcessedReferralsCleaner(ctx)
	}
	prc.shutdown = closeAll(mbConsumer, prc.mb, prc.db, prc.DeviceMetadataRepository.Close)

	return prc
}

func (r *repository) Close() error {
	return errors.Wrap(r.shutdown(), "closing users repository failed")
}

func closeAll(mbConsumer, mbProducer messagebroker.Client, db *storage.DB, otherClosers ...func() error) func() error {
	return func() error {
		var err1 error
		if mbConsumer != nil {
			err1 = errors.Wrap(mbConsumer.Close(), "closing message broker consumer connection failed")
		}
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
	if err := p.db.Ping(ctx); err != nil {
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

func runConcurrently[ARG any](ctx context.Context, run func(context.Context, ARG) error, args []ARG) error {
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "unexpected deadline")
	}
	if len(args) == 0 {
		return nil
	}
	wg := new(sync.WaitGroup)
	wg.Add(len(args))
	errChan := make(chan error, len(args))
	for i := range args {
		go func(ix int) {
			defer wg.Done()
			errChan <- errors.Wrapf(run(ctx, args[ix]), "failed to run:%#v", args[ix])
		}(i)
	}
	wg.Wait()
	close(errChan)
	errs := make([]error, 0, len(args))
	for err := range errChan {
		errs = append(errs, err)
	}

	return errors.Wrap(multierror.Append(nil, errs...).ErrorOrNil(), "at least one execution failed")
}

func randomBetween(left, right uint64) uint64 {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(right)-int64(left)))
	log.Panic(errors.Wrap(err, "crypto random generator failed"))

	return n.Uint64() + left
}

func requestingUserID(ctx context.Context) (requestingUserID string) {
	requestingUserID, _ = ctx.Value(RequestingUserIDCtxValueKey).(string) //nolint:errcheck,revive // Not needed.

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

func ContextWithAuthorization(ctx context.Context, authorization string) context.Context {
	if authorization == "" {
		return ctx
	}

	return context.WithValue(ctx, authorizationCtxValueKey, authorization) //nolint:revive,staticcheck // Not an issue.
}

func authorization(ctx context.Context) (authorization string) {
	authorization, _ = ctx.Value(authorizationCtxValueKey).(string) //nolint:errcheck,revive // Not needed.

	return
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

func (e *Enum[T]) SetDimensions(dimensions []pgtype.ArrayDimension) error {
	if len(dimensions) == 0 {
		*e = nil

		return nil
	}

	*e = make(Enum[T], dimensions[0].Length)

	return nil
}

func (e Enum[T]) ScanIndex(i int) any {
	return &e[i]
}

func (Enum[T]) ScanIndexType() any {
	return new(T)
}

func (j *JSON) Scan(src any) error {
	valBytes, isBytes := src.([]byte)
	if !isBytes {
		val, isStr := src.(string)
		if !isStr {
			return errors.Errorf("unexpected type for src:%#v(%T)", src, src)
		}
		if val == "" {
			return nil
		}
		if val == "{}" {
			*j = make(JSON, 0)
		}
		valBytes = []byte(val)
	}
	if len(valBytes) > 2 { //nolint:gomnd // {}
		return errors.Wrapf(json.UnmarshalContext(context.Background(), valBytes, j), "failed to json.Unmarshall(%v,*JSON)", string(valBytes))
	}

	return nil
}

func (u *User) Checksum() string {
	if u.UpdatedAt == nil {
		return ""
	}
	const base10 = 10

	return strconv.FormatInt(u.UpdatedAt.UnixNano(), base10)
}

func (r *repository) sanitizeUserForUI(usr *User) {
	usr.RandomReferredBy = nil
	usr.PhoneNumberHash = ""
	usr.HashCode = 0
	if usr.Username == usr.ID {
		usr.Username = ""
	}
	if usr.ReferredBy == usr.ID {
		usr.ReferredBy = ""
	}
	r.buildRepeatableKYCSteps(usr)
}

func (r *repository) buildRepeatableKYCSteps(usr *User) {
	if usr.KYCStepPassed == nil ||
		*usr.KYCStepPassed < LivenessDetectionKYCStep ||
		usr.KYCStepsLastUpdatedAt == nil ||
		len(*usr.KYCStepsLastUpdatedAt) < int(LivenessDetectionKYCStep) {
		return
	}
	if r.cfg.IntervalBetweenRepeatableKYCSteps == 0 {
		log.Panic(errors.New("`intervalBetweenRepeatableKYCSteps` config is missing"))
	}
	nextDate := (*usr.KYCStepsLastUpdatedAt)[LivenessDetectionKYCStep-1].Add(r.cfg.IntervalBetweenRepeatableKYCSteps)
	repeatableKYCSteps := make(map[KYCStep]*time.Time, 1)
	repeatableKYCSteps[LivenessDetectionKYCStep] = time.New(nextDate)
	usr.RepeatableKYCSteps = &repeatableKYCSteps
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

func generateUsernameKeywords(username string) []string {
	if username == "" {
		return nil
	}
	keywordsMap := make(map[string]struct{})
	for _, part := range append(strings.Split(username, "."), username) {
		for i := 0; i < len(part); i++ {
			keywordsMap[part[:i+1]] = struct{}{}
			keywordsMap[part[len(part)-1-i:]] = struct{}{}
		}
	}
	keywords := make([]string, 0, len(keywordsMap))
	for keyword := range keywordsMap {
		keywords = append(keywords, keyword)
	}

	return keywords
}

func ConfirmedEmailContext(ctx context.Context, emailValue string) context.Context {
	return context.WithValue(ctx, confirmedEmailCtxValueKey, emailValue) //nolint:revive,staticcheck // .
}

func ConfirmedEmail(ctx context.Context) string {
	email, ok := ctx.Value(confirmedEmailCtxValueKey).(string)
	if ok {
		return email
	}

	return ""
}
