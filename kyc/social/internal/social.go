// SPDX-License-Identifier: ice License 1.0

package social

import (
	"os"

	appcfg "github.com/ice-blockchain/wintr/config"
	"github.com/ice-blockchain/wintr/log"
)

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

func New(st StrategyType) Verifier {
	conf := loadConfig()
	sc := newMustWebScraper(conf.WebScrapingAPI.URL, conf.WebScrapingAPI.APIKey)

	switch st {
	case StrategyTwitter:
		return newTwitterVerifier(sc, conf.SocialLinks.Twitter.PostURL, conf.SocialLinks.Twitter.Domains)

	case StrategyFacebook:
		return newFacebookVerifier(sc)

	default:
		log.Panic("invalid social verifier: " + st)
	}

	return nil
}
