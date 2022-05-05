// SPDX-License-Identifier: BUSL-1.1

package ip2location

import (
	"context"
	"io"

	"github.com/ip2location/ip2location-go"
)

// Public API.

type (
	IP = string

	Repository interface {
		io.Closer
		GetCountry(context.Context, IP) string
	}
)

// Private API.

const applicationYamlKey = "ip2location"

type ip2locationRepository struct {
	db *ip2location.DB
}

// | config holds the configuration of this package mounted from `application.yaml`.
type config struct {
	BinaryLocation string `yaml:"binaryLocation"`
}

//nolint:gochecknoglobals // Because its loaded once, at runtime.
var cfg config
