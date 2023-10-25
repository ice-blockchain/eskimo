// SPDX-License-Identifier: ice License 1.0

package social

import (
	"os"

	appcfg "github.com/ice-blockchain/wintr/config"
)

//nolint:gochecknoinits // .
func init() { // Remove this asap.
	loadConfig()
}

func loadConfig() *config {
	var cfg config
	appcfg.MustLoadFromKey(applicationYAMLKey, &cfg)
	if cfg.WebScrapingAPI.APIKey == "" {
		cfg.WebScrapingAPI.APIKey = os.Getenv("WEB_SCRAPING_API_KEY")
	}
	if cfg.WebScrapingAPI.URL == "" {
		cfg.WebScrapingAPI.URL = os.Getenv("WEB_SCRAPING_API_URL")
	}

	return &cfg
}
