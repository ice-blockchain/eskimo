// SPDX-License-Identifier: BUSL-1.1

package countries

import (
	"context"
	_ "embed"
	"io"

	"github.com/ip2location/ip2location-go"
)

// Public API.

type (
	Country = string
	IP      = string

	Repository interface {
		io.Closer
		Get(context.Context, IP) string
	}
)

// Private API.

const applicationYamlKey = "countries"

//nolint:gochecknoglobals // Because its loaded once, at runtime.
var (
	cfg       config
	countries map[string]bool
)

//go:embed countrycodes.map
var countriesList string

type ip2locationRepository struct {
	db *ip2location.DB
}

// | config holds the configuration of this package mounted from `application.yaml`.
type config struct {
	BinaryLocation string `yaml:"binaryLocation"`
}
