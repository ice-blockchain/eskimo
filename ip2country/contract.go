// SPDX-License-Identifier: BUSL-1.1

package ip2country

import (
	"context"
	"io"

	"github.com/ip2location/ip2location-go"
)

type Repository interface {
	io.Closer
	GetCountry(context.Context, string) string
}

type IPDatabase struct {
	db *ip2location.DB
}
