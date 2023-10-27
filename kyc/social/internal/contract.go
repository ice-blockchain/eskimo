// SPDX-License-Identifier: ice License 1.0

package social

import (
	"context"

	"github.com/pkg/errors"
)

// Private API.

const (
	applicationYAMLKey = "kyc/social"
)

type (
	StrategyType string

	Metadata struct {
		AccessToken string
	}

	Verifier interface {
		VerifyPost(ctx context.Context, metadata *Metadata, postURL, expectedPostText string) (username string, err error)
	}
)

type (
	webScraperOptionsFunc func(map[string]string) map[string]string

	webScraper interface {
		Scrape(ctx context.Context, url string, opts webScraperOptionsFunc) (content []byte, err error)
	}

	webScraperImpl struct {
		ScrapeAPIURL string
		APIKey       string
	}

	twitterVerifierImpl struct {
		Scraper webScraper
		Post    string
		Domains []string
	}

	facebookVerifierImpl struct {
		Scraper webScraper
	}

	configTwitter struct {
		PostURL string   `yaml:"post-url" mapstructure:"post-url"` //nolint:tagliatelle // Nope.
		Domains []string `yaml:"domains"  mapstructure:"domains"`
	}

	config struct {
		WebScrapingAPI struct {
			APIKey string `yaml:"api-key" mapstructure:"api-key"` //nolint:tagliatelle // Nope.
			URL    string `yaml:"url"     mapstructure:"url"`
		} `yaml:"web-scraping-api" mapstructure:"web-scraping-api"` //nolint:tagliatelle // Nope.

		SocialLinks struct {
			Twitter configTwitter `yaml:"twitter"  mapstructure:"twitter"`
		} `yaml:"social-links" mapstructure:"social-links"` //nolint:tagliatelle // Nope.
	}
)

const (
	StrategyFacebook StrategyType = "facebook"
	StrategyTwitter  StrategyType = "twitter"
)

var (
	ErrInvalidPageContent = errors.New("invalid page content")
	ErrTextNotFound       = errors.New("expected text not found")
	ErrUsernameNotFound   = errors.New("username not found")
	ErrPostNotFound       = errors.New("post not found")
	ErrInvalidURL         = errors.New("invalid URL")
	ErrFetchFailed        = errors.New("cannot fetch post")
	ErrFetchReadFailed    = errors.New("cannot read fetched post")
	ErrScrapeFailed       = errors.New("cannot scrape target")
	ErrUnavailable        = errors.New("service unavailable")
)
