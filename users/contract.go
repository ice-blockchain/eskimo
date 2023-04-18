// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	_ "embed"
	"io"
	"mime/multipart"
	"net"
	"regexp"
	stdlibtime "time"

	"github.com/pkg/errors"

	"github.com/ice-blockchain/eskimo/users/internal/device"
	devicemetadata "github.com/ice-blockchain/eskimo/users/internal/device/metadata"
	"github.com/ice-blockchain/wintr/analytics/tracking"
	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	storage "github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/multimedia/picture"
	"github.com/ice-blockchain/wintr/time"
)

// Public API.

const (
	UsernameRegex = `^[.a-zA-Z0-9]{4,30}$`
)

const (
	GlobalRankHiddenProfileElement    HiddenProfileElement = "globalRank"
	ReferralCountHiddenProfileElement HiddenProfileElement = "referralCount"
	LevelHiddenProfileElement         HiddenProfileElement = "level"
	RoleHiddenProfileElement          HiddenProfileElement = "role"
	BadgesHiddenProfileElement        HiddenProfileElement = "badges"
)

const (
	ContactsReferrals ReferralType = "CONTACTS"
	Tier1Referrals    ReferralType = "T1"
	Tier2Referrals    ReferralType = "T2"
)

var (
	ErrNotFound           = storage.ErrNotFound
	ErrRelationNotFound   = storage.ErrRelationNotFound
	ErrDuplicate          = storage.ErrDuplicate
	ErrInvalidAppVersion  = devicemetadata.ErrInvalidAppVersion
	ErrOutdatedAppVersion = devicemetadata.ErrOutdatedAppVersion
	ErrInvalidCountry     = errors.New("country invalid")
	ErrRaceCondition      = errors.New("race condition")
	//nolint:gochecknoglobals // It's just for more descriptive validation messages.
	ReferralTypes = Enum[ReferralType]{ContactsReferrals, Tier1Referrals, Tier2Referrals}
	//nolint:gochecknoglobals // It's just for more descriptive validation messages.
	HiddenProfileElements = Enum[HiddenProfileElement]{
		GlobalRankHiddenProfileElement,
		ReferralCountHiddenProfileElement,
		LevelHiddenProfileElement,
		RoleHiddenProfileElement,
		BadgesHiddenProfileElement,
	}
	CompiledUsernameRegex = regexp.MustCompile(UsernameRegex)
)

type (
	ReferralType             string
	HiddenProfileElement     string
	NotExpired               bool
	Enum[T ~string]          []T
	JSON                     map[string]any
	UserID                   = string
	SensitiveUserInformation struct {
		PhoneNumber string `json:"phoneNumber,omitempty" example:"+12099216581" swaggertype:"string"`
		Email       string `json:"email,omitempty" example:"jdoe@gmail.com" swaggertype:"string"`
	}
	PrivateUserInformation struct {
		SensitiveUserInformation
		FirstName string `json:"firstName,omitempty" example:"John" `
		LastName  string `json:"lastName,omitempty" example:"Doe"`
		devicemetadata.DeviceLocation
	}
	PublicUserInformation struct {
		ID                UserID `json:"id,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Username          string `json:"username,omitempty" example:"jdoe"`
		ProfilePictureURL string `json:"profilePictureUrl,omitempty" example:"https://somecdn.com/p1.jpg"`
	}
	User struct {
		CreatedAt               *time.Time                  `json:"createdAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		UpdatedAt               *time.Time                  `json:"updatedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		LastMiningStartedAt     *time.Time                  `json:"lastMiningStartedAt,omitempty" example:"2022-01-03T16:20:52.156534Z" swaggerignore:"true"`
		LastMiningEndedAt       *time.Time                  `json:"lastMiningEndedAt,omitempty" example:"2022-01-03T16:20:52.156534Z" swaggerignore:"true"`
		LastPingCooldownEndedAt *time.Time                  `json:"lastPingCooldownEndedAt,omitempty" example:"2022-01-03T16:20:52.156534Z" swaggerignore:"true"`
		HiddenProfileElements   *Enum[HiddenProfileElement] `json:"hiddenProfileElements,omitempty" swaggertype:"array,string" example:"level" enums:"globalRank,referralCount,level,role,badges"` //nolint:lll // .
		RandomReferredBy        *bool                       `json:"randomReferredBy,omitempty" example:"true" swaggerignore:"true"`
		Verified                *bool                       `json:"-" example:"true" swaggerignore:"true"`
		ClientData              *JSON                       `json:"clientData,omitempty"`
		PrivateUserInformation
		PublicUserInformation
		ReferredBy                     UserID `json:"referredBy,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2" `
		PhoneNumberHash                string `json:"phoneNumberHash,omitempty" example:"Ef86A6021afCDe5673511376B2" swaggerignore:"true"`
		AgendaPhoneNumberHashes        string `json:"agendaPhoneNumberHashes,omitempty" example:"Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2"` //nolint:lll // .
		MiningBlockchainAccountAddress string `json:"miningBlockchainAccountAddress,omitempty" example:"0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		BlockchainAccountAddress       string `json:"blockchainAccountAddress,omitempty" example:"0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Language                       string `json:"language,omitempty" example:"en"`
		HashCode                       int64  `json:"hashCode,omitempty" example:"43453546464576547" swaggerignore:"true"`
	}
	MinimalUserProfile struct {
		Active *NotExpired `json:"active,omitempty" example:"true"`
		Pinged *NotExpired `json:"pinged,omitempty" example:"false"`
		SensitiveUserInformation
		PublicUserInformation
		devicemetadata.DeviceLocation
		ReferralType ReferralType `json:"referralType,omitempty" example:"T1" enums:"CONTACTS,T0,T1,T2"`
	}
	UserProfile struct {
		*User
		T1ReferralCount *uint64 `json:"t1ReferralCount,omitempty" example:"100"`
		T2ReferralCount *uint64 `json:"t2ReferralCount,omitempty" example:"100"`
	}
	Referrals struct {
		Referrals []*MinimalUserProfile `json:"referrals"`
		UserCount
	}
	UserSnapshot struct {
		*User
		Before *User `json:"before,omitempty"`
	}
	ReferralAcquisition struct {
		Date *time.Time `json:"date" example:"2022-01-03"`
		T1   uint64     `json:"t1" example:"22"`
		T2   uint64     `json:"t2" example:"13"`
	}
	CountryStatistics struct {
		// ISO 3166 country code.
		Country   devicemetadata.Country `json:"country" example:"US"`
		UserCount uint64                 `json:"userCount" example:"12121212"`
	}
	UserCount struct {
		Active uint64 `json:"active" example:"11"`
		Total  uint64 `json:"total" example:"11"`
	}
	UserCountTimeSeriesDataPoint struct {
		Date *time.Time `json:"date" example:"2022-01-03T16:20:52.156534Z"`
		UserCount
	}
	UserGrowthStatistics struct {
		TimeSeries []*UserCountTimeSeriesDataPoint `json:"timeSeries"`
		UserCount
	}
	PhoneNumberValidation struct {
		// `Read Only`.
		CreatedAt       *time.Time `json:"createdAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		UserID          UserID     `json:"userId,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		PhoneNumber     string     `json:"phoneNumber,omitempty" example:"+12345678"`
		PhoneNumberHash string     `json:"phoneNumberHash,omitempty" example:"Ef86A6021afCDe5673511376B2"`
		ValidationCode  string     `json:"validationCode,omitempty" example:"1234"`
	}
	EmailValidation struct {
		// `Read Only`.
		CreatedAt      *time.Time `json:"createdAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		UserID         UserID     `json:"userId,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Email          string     `json:"email,omitempty" example:"someone1@example.com"`
		ValidationCode string     `json:"validationCode,omitempty" example:"1234"`
	}
	GlobalUnsigned struct {
		Key   string `json:"key" example:"TOTAL_USERS_2022-01-22:16"`
		Value uint64 `json:"value" example:"123676"`
	}
	ReadRepository interface {
		GetUsers(ctx context.Context, keyword string, limit, offset uint64) ([]*MinimalUserProfile, error)
		GetUserByUsername(ctx context.Context, username string) (*UserProfile, error)
		GetUserByID(ctx context.Context, userID string) (*UserProfile, error)

		GetTopCountries(ctx context.Context, keyword string, limit, offset uint64) ([]*CountryStatistics, error)
		GetUserGrowth(ctx context.Context, days uint64) (*UserGrowthStatistics, error)

		GetReferrals(ctx context.Context, userID string, referralType ReferralType, limit, offset uint64) (*Referrals, error)
		GetReferralAcquisitionHistory(ctx context.Context, userID string, days uint64) ([]*ReferralAcquisition, error)
	}
	WriteRepository interface {
		CreateUser(ctx context.Context, usr *User, clientIP net.IP) error
		DeleteUser(ctx context.Context, userID UserID) error
		ModifyUser(ctx context.Context, usr *User, profilePicture *multipart.FileHeader) error
	}
	// Repository main API exposed that handles all the features of this package.
	Repository interface {
		io.Closer
		devicemetadata.DeviceMetadataRepository

		ReadRepository
		WriteRepository
	}
	Processor interface {
		Repository
		CheckHealth(context.Context) error
	}
)

// Proxy Internal Types.
type (
	DeviceID               = device.ID
	DeviceMetadataSnapshot = devicemetadata.DeviceMetadataSnapshot
	DeviceMetadata         = devicemetadata.DeviceMetadata
	DeviceLocation         = devicemetadata.DeviceLocation
)

// Private API.

const (
	applicationYamlKey                  = "users"
	dayFormat, hourFormat, minuteFormat = "2006-01-02", "2006-01-02T15", "2006-01-02T15:04"
	totalUsersGlobalKey                 = "TOTAL_USERS"
	totalActiveUsersGlobalKey           = "TOTAL_ACTIVE_USERS"
	checksumCtxValueKey                 = "versioningChecksumCtxValueKey"
	requestingUserIDCtxValueKey         = "requestingUserIDCtxValueKey"
	totalNoOfDefaultProfilePictures     = 20
	defaultProfilePictureName           = "default-profile-picture-%v.png"
	defaultProfilePictureNameRegex      = "default-profile-picture-\\d+[.]png"
	hashCodeDBColumnName                = "hash_code"
	usernameDBColumnName                = "username"
	requestDeadline                     = 25 * stdlibtime.Second

	agendaPhoneNumberHashesBatchSize = 500
)

//go:embed DDL.sql
var ddl string //nolint:grouper // .

type (
	miningSession struct {
		EndedAt *time.Time `json:"endedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		UserID  string     `json:"userId,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
	}

	userSnapshotSource struct {
		*processor
	}
	miningSessionSource struct {
		*processor
	}
	userPingSource struct {
		*processor
	}

	// | repository implements the public API that this package exposes.
	repository struct {
		cfg *config
		db  *storage.DB
		mb  messagebroker.Client
		devicemetadata.DeviceMetadataRepository
		pictureClient  picture.Client
		trackingClient tracking.Client
		shutdown       func() error
	}

	processor struct {
		*repository
	}
	// | config holds the configuration of this package mounted from `application.yaml`.
	config struct {
		PhoneNumberValidation struct {
			SmsTemplate    string              `yaml:"smsTemplate"`
			ExpirationTime stdlibtime.Duration `yaml:"expirationTime"`
		} `yaml:"phoneNumberValidation"`
		EmailValidation struct {
			FromEmailName         string              `yaml:"fromEmailName"`
			FromEmailAddress      string              `yaml:"fromEmailAddress"`
			EmailBodyHTMLTemplate string              `mapstructure:"emailBodyHTMLTemplate" yaml:"emailBodyHTMLTemplate"` //nolint:tagliatelle // Nope.
			EmailSubject          string              `yaml:"emailSubject"`
			ExpirationTime        stdlibtime.Duration `yaml:"expirationTime"`
		} `yaml:"emailValidation"`
		messagebroker.Config      `mapstructure:",squash"` //nolint:tagliatelle // Nope.
		GlobalAggregationInterval struct {
			Parent stdlibtime.Duration `yaml:"parent"`
			Child  stdlibtime.Duration `yaml:"child"`
		} `yaml:"globalAggregationInterval"`
	}
)
