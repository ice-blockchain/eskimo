// SPDX-License-Identifier: ice License 1.0

package fixture

import (
	stdlibtime "time"

	serverfixture "github.com/ice-blockchain/wintr/server/fixture"
)

// Public API.

type (
	TestConnectorsBridge struct {
		R                        serverfixture.TestConnector
		W                        serverfixture.TestConnector
		TimeRegex                string
		DefaultClientIP          string
		DefaultClientIPCountry   string
		DefaultClientIPCity      string
		DefaultProfilePictureURL string
		TestDeadline             stdlibtime.Duration
	}
)

// Private API.

const (
	testDeadline             = 30 * stdlibtime.Second
	timeRegex                = "[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[1-2][0-9]|3[0-1])T(2[0-3]|[01][0-9]):[0-5][0-9]:([0-9]+)[.]([0-9]+)Z"
	defaultClientIP          = "1.1.1.1"
	defaultClientIPCountry   = "US"
	defaultClientIPCity      = "Los Angeles"
	defaultProfilePictureURL = "https[:][/][/]ice-staging[.]b-cdn[.]net[/]profile[/]default-profile-picture-\\d+[.]png"
)
