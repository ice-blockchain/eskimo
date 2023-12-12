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

	for ptr, env := range map[*string]string{
		&cfg.WebScrapingAPI.APIKey:          os.Getenv("WEB_SCRAPING_API_KEY"),
		&cfg.WebScrapingAPI.URL:             os.Getenv("WEB_SCRAPING_API_URL"),
		&cfg.SocialLinks.Facebook.AppID:     os.Getenv("FACEBOOK_APP_ID"),
		&cfg.SocialLinks.Facebook.AppSecret: os.Getenv("FACEBOOK_APP_SECRET"),
	} {
		if *ptr == "" {
			*ptr = env
		}
	}

	return &cfg
}

func New(st StrategyType) Verifier {
	conf := loadConfig()

	switch st {
	case StrategyTwitter:
		sc := newMustWebScraper(conf.WebScrapingAPI.URL, conf.WebScrapingAPI.APIKey)

		return newTwitterVerifier(sc, conf.SocialLinks.Twitter.Domains, conf.SocialLinks.Twitter.Countries)

	case StrategyFacebook:
		sc := new(dataFetcherImpl)

		return newFacebookVerifier(
			sc,
			conf.SocialLinks.Facebook.AppID,
			conf.SocialLinks.Facebook.AppSecret,
			conf.SocialLinks.Facebook.AllowLongLiveTokens,
		)

	default:
		log.Panic("invalid social verifier: " + st)
	}

	return nil
}
