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

	if cfg.SocialLinks.Facebook.AppID == "" {
		cfg.SocialLinks.Facebook.AppID = os.Getenv("FACEBOOK_APP_ID")
	}

	if cfg.SocialLinks.Facebook.AppSecret == "" {
		cfg.SocialLinks.Facebook.AppSecret = os.Getenv("FACEBOOK_APP_SECRET")
	}

	return &cfg
}

func New(st StrategyType) Verifier {
	conf := loadConfig()

	switch st {
	case StrategyTwitter:
		sc := newMustWebScraper(conf.WebScrapingAPI.URL, conf.WebScrapingAPI.APIKey)

		return newTwitterVerifier(sc, conf.SocialLinks.Twitter.PostURL, conf.SocialLinks.Twitter.Domains)

	case StrategyFacebook:
		sc := new(nativeScraperImpl)

		return newFacebookVerifier(sc, conf.SocialLinks.Facebook.AppID, conf.SocialLinks.Facebook.AppSecret)

	default:
		log.Panic("invalid social verifier: " + st)
	}

	return nil
}
