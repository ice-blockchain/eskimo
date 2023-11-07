// SPDX-License-Identifier: ice License 1.0

package social

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/imroc/req/v3"

	"github.com/ice-blockchain/wintr/log"
)

const (
	scrapeHTTPMaxRetries = 3
)

func (*dataFetcherImpl) Fetch(ctx context.Context, target string) ([]byte, error) {
	resp, err := req.DefaultClient().
		R().
		SetContext(ctx).
		SetRetryBackoffInterval(10*time.Millisecond, time.Second). //nolint:gomnd // Nope.
		SetRetryCount(scrapeHTTPMaxRetries).
		SetRetryHook(func(resp *req.Response, err error) {
			if err != nil {
				log.Error(err, "scaper: fetch failed")
			} else {
				log.Warn("scaper: fetch failed: unexpected status code: " + resp.Status)
			}
		}).
		SetRetryCondition(func(resp *req.Response, err error) bool {
			return !(err == nil && resp.GetStatusCode() == http.StatusOK)
		}).
		Get(target)
	if err != nil {
		return nil, multierror.Append(ErrFetchFailed, err)
	}

	data, err := resp.ToBytes()
	if err != nil {
		return nil, multierror.Append(ErrFetchReadFailed, err)
	}

	return data, nil
}

func (s *webScraperImpl) Scrape(ctx context.Context, target string, options webScraperOptionsFunc) ([]byte, error) {
	return s.Fetcher.Fetch(ctx, s.BuildQuery(target, options)) //nolint:wrapcheck // False-Positive.
}

func (*webScraperImpl) randomCountry() string {
	countries := []string{"US", "CA", "MX"}

	return countries[time.Now().UnixNano()%int64(len(countries))]
}

func (s *webScraperImpl) BuildQuery(target string, options webScraperOptionsFunc) string {
	conf := map[string]string{
		"render_js":  "1",
		"device":     "mobile",
		"proxy_type": "residential",
		"timeout":    "30000",
		"country":    s.randomCountry(),
		"wait_until": "networkidle2",
	}

	if options != nil {
		conf = options(conf)
	}

	parsed, err := url.Parse(s.ScrapeAPIURL)
	if err != nil {
		log.Panic("scaper: invalid URL: " + err.Error())
	}

	query := parsed.Query()
	for k, v := range conf {
		query.Set(k, v)
	}

	query.Set("api_key", s.APIKey)
	query.Set("url", target)
	parsed.RawQuery = query.Encode()

	return parsed.String()
}

func newMustWebScraper(apiURL, apiKey string) webScraper {
	if apiURL == "" {
		log.Panic("scaper: URL is not set")
	}

	return &webScraperImpl{
		ScrapeAPIURL: apiURL,
		APIKey:       apiKey,
		Fetcher:      new(dataFetcherImpl),
	}
}
