// SPDX-License-Identifier: ice License 1.0

package social

import (
	"net/url"
)

func hasRootDomainAndHTTPS(targetURL, expectedDomain string) bool {
	const expectedScheme = "https"

	parsed, err := url.Parse(targetURL)
	if err != nil {
		return false
	}

	if parsed.Scheme != expectedScheme {
		return false
	}

	host := parsed.Hostname()
	if len(host) < len(expectedDomain) || host[len(host)-len(expectedDomain):] != expectedDomain {
		return false
	}

	return expectedDomain == host || host[len(host)-len(expectedDomain)-1] == '.'
}
