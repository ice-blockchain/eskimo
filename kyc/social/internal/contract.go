// SPDX-License-Identifier: ice License 1.0

package social

// Private API.

const (
	applicationYAMLKey = "kyc/social"
)

type (
	config struct {
		WebScrapingAPI struct {
			APIKey string `yaml:"api-key" mapstructure:"api-key"` //nolint:tagliatelle // Nope.
			URL    string `yaml:"url" mapstructure:"url"`
		} `yaml:"web-scraping-api" mapstructure:"web-scraping-api"` //nolint:tagliatelle // Nope.
	}
)
