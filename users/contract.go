// SPDX-License-Identifier: ice License 1.0

package users

import (
	"context"
	"database/sql"
	_ "embed"
	"io"
	"mime/multipart"
	"net"
	"regexp"
	stdlibtime "time"

	"github.com/jackc/pgx/v5/pgtype"
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
	UsernameRegex               = `^[.a-zA-Z0-9]{4,30}$`
	RequestingUserIDCtxValueKey = "requestingUserIDCtxValueKey"
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
	TeamReferrals     ReferralType = "TEAM"
)

const (
	NoneKYCStep KYCStep = iota
	FacialRecognitionKYCStep
	LivenessDetectionKYCStep
	Social1KYCStep
	QuizKYCStep
	Social2KYCStep
	Social3KYCStep
	Social4KYCStep
	Social5KYCStep
	Social6KYCStep
	Social7KYCStep
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
	ReferralTypes = Enum[ReferralType]{ContactsReferrals, Tier1Referrals, Tier2Referrals, TeamReferrals}
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
	KYCStep                  int8
	ReferralType             string
	HiddenProfileElement     string
	NotExpired               bool
	Enum[T ~string]          []T
	JSON                     map[string]any
	UserID                   = string
	SensitiveUserInformation struct {
		PhoneNumber string `json:"phoneNumber,omitempty" example:"+12099216581" swaggertype:"string" db:"phone_number"`
		Email       string `json:"email,omitempty" example:"jdoe@gmail.com" swaggertype:"string" db:"email"`
	}
	PrivateUserInformation struct {
		SensitiveUserInformation
		FirstName *string `json:"firstName,omitempty" example:"John" db:"first_name"`
		LastName  *string `json:"lastName,omitempty" example:"Doe" db:"last_name"`
		devicemetadata.DeviceLocation
		PreStaking
	}
	PublicUserInformation struct {
		ID                UserID `json:"id,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2" db:"id"`
		Username          string `json:"username,omitempty" example:"jdoe" db:"username"`
		ProfilePictureURL string `json:"profilePictureUrl,omitempty" example:"https://somecdn.com/p1.jpg" db:"profile_picture_name"`
	}
	PreStaking struct {
		Years      uint64  `json:"years,omitempty" swaggerignore:"true" example:"1" db:"pre_staking_years"`
		Allocation float64 `json:"allocation,omitempty" swaggerignore:"true" example:"100.00" db:"pre_staking_allocation"`
		Bonus      float64 `json:"bonus,omitempty" swaggerignore:"true" example:"100.00" db:"pre_staking_bonus"`
	}
	PreStakingSummary struct {
		UserID string `json:"userId,omitempty" example:"edfd8c02-75e0-4687-9ac2-1ce4723865c4"`
		PreStaking
	}
	preStakingSnapshot struct {
		Before *PreStakingSummary `json:"before,omitempty"`
		PreStakingSummary
	}
	User struct {
		CreatedAt               *time.Time                  `json:"createdAt,omitempty" example:"2022-01-03T16:20:52.156534Z" db:"created_at"`
		UpdatedAt               *time.Time                  `json:"updatedAt,omitempty" example:"2022-01-03T16:20:52.156534Z" db:"updated_at"`
		LastMiningStartedAt     *time.Time                  `json:"lastMiningStartedAt,omitempty" example:"2022-01-03T16:20:52.156534Z" swaggerignore:"true" db:"last_mining_started_at"`                                       //nolint:lll // .
		LastMiningEndedAt       *time.Time                  `json:"lastMiningEndedAt,omitempty" example:"2022-01-03T16:20:52.156534Z" swaggerignore:"true" db:"last_mining_ended_at"`                                           //nolint:lll // .
		LastPingCooldownEndedAt *time.Time                  `json:"lastPingCooldownEndedAt,omitempty" example:"2022-01-03T16:20:52.156534Z" swaggerignore:"true" db:"last_ping_cooldown_ended_at"`                              //nolint:lll // .
		HiddenProfileElements   *Enum[HiddenProfileElement] `json:"hiddenProfileElements,omitempty" swaggertype:"array,string" example:"level" enums:"globalRank,referralCount,level,role,badges" db:"hidden_profile_elements"` //nolint:lll // .
		RandomReferredBy        *bool                       `json:"randomReferredBy,omitempty" example:"true" swaggerignore:"true" db:"random_referred_by"`
		Verified                *bool                       `json:"verified,omitempty" example:"true" db:"-"`
		KYCStepsLastUpdatedAt   *[]*time.Time               `json:"kycStepsLastUpdatedAt,omitempty" swaggertype:"array,string" example:"2022-01-03T16:20:52.156534Z" db:"kyc_steps_last_updated_at"` //nolint:lll // .
		KYCStepsCreatedAt       *[]*time.Time               `json:"kycStepsCreatedAt,omitempty" swaggertype:"array,string" example:"2022-01-03T16:20:52.156534Z" db:"kyc_steps_created_at"`          //nolint:lll // .
		KYCStepPassed           *KYCStep                    `json:"kycStepPassed,omitempty" example:"0" db:"kyc_step_passed"`
		KYCStepBlocked          *KYCStep                    `json:"kycStepBlocked,omitempty" example:"0" db:"kyc_step_blocked"`
		ClientData              *JSON                       `json:"clientData,omitempty" db:"client_data"`
		RepeatableKYCSteps      *map[KYCStep]*time.Time     `json:"repeatableKYCSteps,omitempty" db:"-"` //nolint:tagliatelle // Nope.
		PublicUserInformation
		ReferredBy                           UserID   `json:"referredBy,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2" db:"referred_by"`
		PhoneNumberHash                      string   `json:"phoneNumberHash,omitempty" example:"Ef86A6021afCDe5673511376B2" swaggerignore:"true" db:"phone_number_hash"`
		AgendaPhoneNumberHashes              *string  `json:"agendaPhoneNumberHashes,omitempty" example:"Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2,Ef86A6021afCDe5673511376B2" db:"-"` //nolint:lll // .
		MiningBlockchainAccountAddress       string   `json:"miningBlockchainAccountAddress,omitempty" example:"0x4B73C58370AEfcEf86A6021afCDe5673511376B2" db:"mining_blockchain_account_address"`                           //nolint:lll // .
		SolanaMiningBlockchainAccountAddress string   `json:"solanaMiningBlockchainAccountAddress,omitempty" example:"0x4B73C58370AEfcEf86A6021afCDe5673511376B2" db:"solana_mining_blockchain_account_address"`              //nolint:lll // .
		BlockchainAccountAddress             string   `json:"blockchainAccountAddress,omitempty" example:"0x4B73C58370AEfcEf86A6021afCDe5673511376B2" db:"blockchain_account_address"`                                        //nolint:lll // .
		Language                             string   `json:"language,omitempty" example:"en" db:"language"`
		Lookup                               string   `json:"-" example:"username" db:"lookup"`
		AgendaContactUserIDs                 []string `json:"agendaContactUserIDs,omitempty" swaggerignore:"true" db:"agenda_contact_user_ids"`
		PrivateUserInformation
		HashCode int64 `json:"hashCode,omitempty" example:"43453546464576547" swaggerignore:"true" db:"hash_code"`
	}
	MinimalUserProfile struct {
		Verified *bool       `json:"verified,omitempty" example:"true"`
		Active   *NotExpired `json:"active,omitempty" example:"true"`
		Pinged   *NotExpired `json:"pinged,omitempty" example:"false"`
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
	GlobalUnsigned struct {
		Key   string `json:"key" example:"TOTAL_USERS_2022-01-22:16"`
		Value uint64 `json:"value" example:"123676"`
	}
	Contact struct {
		UserID        UserID `json:"userId,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		ContactUserID UserID `json:"contactUserId,omitempty" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
	}
	ReadRepository interface {
		GetUsers(ctx context.Context, keyword string, limit, offset uint64) ([]*MinimalUserProfile, error)
		GetUserByUsername(ctx context.Context, username string) (*UserProfile, error)
		GetUserByPhoneNumber(ctx context.Context, phoneNumber string) (*User, error)
		GetUserByID(ctx context.Context, userID string) (*UserProfile, error)

		GetTopCountries(ctx context.Context, keyword string, limit, offset uint64) ([]*CountryStatistics, error)
		GetUserGrowth(ctx context.Context, days uint64, tz *stdlibtime.Location) (*UserGrowthStatistics, error)

		GetReferrals(ctx context.Context, userID string, referralType ReferralType, limit, offset uint64) (*Referrals, error)
		GetReferralAcquisitionHistory(ctx context.Context, userID string) ([]*ReferralAcquisition, error)

		IsEmailUsedBySomebodyElse(ctx context.Context, userID, email string) (bool, error)
	}
	WriteRepository interface {
		CreateUser(ctx context.Context, usr *User, clientIP net.IP) error
		DeleteUser(ctx context.Context, userID UserID) error
		ModifyUser(ctx context.Context, usr *User, profilePicture *multipart.FileHeader) error

		TryResetKYCSteps(ctx context.Context, userID string) (*User, error)
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
		CheckHealth(ctx context.Context) error
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
	hoursInOneDay                       = 24
	applicationYamlKey                  = "users"
	dayFormat, hourFormat, minuteFormat = "2006-01-02", "2006-01-02T15", "2006-01-02T15:04"
	totalUsersGlobalKey                 = "TOTAL_USERS"
	totalActiveUsersGlobalKey           = "TOTAL_ACTIVE_USERS"
	checksumCtxValueKey                 = "versioningChecksumCtxValueKey"
	confirmedEmailCtxValueKey           = "confirmedEmailCtxValueKey"
	authorizationCtxValueKey            = "authorizationCtxValueKey"
	xAccountMetadataCtxValueKey         = "xAccountMetadataCtxValueKey"
	totalNoOfDefaultProfilePictures     = 20
	defaultProfilePictureName           = "default-profile-picture-%v.png"
	defaultProfilePictureNameRegex      = "default-profile-picture-\\d+[.]png"
	usernameDBColumnName                = "username"
	requestDeadline                     = 25 * stdlibtime.Second

	maxDaysReferralsHistory = 5
)

var (
	//go:embed DDL.sql
	ddl string

	_ sql.Scanner        = (*JSON)(nil)
	_ sql.Scanner        = (*NotExpired)(nil)
	_ pgtype.ArraySetter = (*Enum[HiddenProfileElement])(nil)
)

type (
	miningSession struct {
		LastNaturalMiningStartedAt *time.Time          `json:"lastNaturalMiningStartedAt,omitempty" example:"2022-01-03T16:20:52.156534Z" swaggerignore:"true"`
		StartedAt                  *time.Time          `json:"startedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		EndedAt                    *time.Time          `json:"endedAt,omitempty" example:"2022-01-03T16:20:52.156534Z"`
		PreviouslyEndedAt          *time.Time          `json:"previouslyEndedAt,omitempty" swaggerignore:"true" example:"2022-01-03T16:20:52.156534Z"`
		UserID                     string              `json:"userId,omitempty" swaggerignore:"true" example:"did:ethr:0x4B73C58370AEfcEf86A6021afCDe5673511376B2"`
		Extension                  stdlibtime.Duration `json:"extension,omitempty" swaggerignore:"true" example:"24h"`
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
	preStakingSource struct {
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
		KYC struct {
			KYCStep1ResetURL string `yaml:"kyc-step1-reset-url" mapstructure:"kyc-step1-reset-url"` //nolint:tagliatelle // Nope.
		} `yaml:"kyc" mapstructure:"kyc"`
		messagebroker.Config      `mapstructure:",squash"` //nolint:tagliatelle // Nope.
		GlobalAggregationInterval struct {
			MinMiningSessionDuration stdlibtime.Duration `yaml:"minMiningSessionDuration"`
			Parent                   stdlibtime.Duration `yaml:"parent"`
			Child                    stdlibtime.Duration `yaml:"child"`
		} `yaml:"globalAggregationInterval"`
		//nolint:tagliatelle // .
		IntervalBetweenRepeatableKYCSteps stdlibtime.Duration `yaml:"intervalBetweenRepeatableKYCSteps" mapstructure:"intervalBetweenRepeatableKYCSteps"`
		DisableConsumer                   bool                `yaml:"disableConsumer"`
	}
)
