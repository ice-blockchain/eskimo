// SPDX-License-Identifier: ice License 1.0

package social

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/imroc/req/v3"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/log"
)

func (c *censorerImpl) Censor(err error) error {
	const censor = "CENSORED"

	if err == nil || c == nil {
		return err
	}

	msg := err.Error()
	for _, s := range c.Strings {
		msg = strings.ReplaceAll(msg, s, censor)
	}

	return errors.New(msg)
}

func (d *dataFetcherImpl) Fetch(ctx context.Context, target string, retry req.RetryConditionFunc) (data []byte, code int, err error) {
	resp, err := req.DefaultClient().
		R().
		SetContext(ctx).
		SetRetryBackoffInterval(0, 0).
		SetRetryCount(0).
		SetRetryHook(func(resp *req.Response, err error) {
			if err != nil {
				log.Error(d.Censorer.Censor(err), "scaper: fetch failed")
			} else {
				log.Warn("scaper: fetch failed: unexpected status code: " + resp.Status)
			}
		}).
		SetRetryCondition(func(resp *req.Response, err error) bool {
			if retry != nil {
				return retry(resp, err)
			}

			return !(err == nil && resp.GetStatusCode() == http.StatusOK)
		}).
		Get(target)
	if err != nil {
		return nil, 0, multierror.Append(ErrFetchFailed, d.Censorer.Censor(err))
	}

	data, err = resp.ToBytes()
	if err != nil {
		return nil, 0, multierror.Append(ErrFetchReadFailed, d.Censorer.Censor(err))
	}

	return data, resp.GetStatusCode(), nil
}

func (s *webScraperImpl) Scrape(ctx context.Context, target string, opts webScraperOptions) (*webScraperResult, error) {
	data, code, err := s.Fetcher.Fetch(ctx, s.BuildQuery(target, opts.ProxyOptions), opts.Retry)
	if err != nil {
		return nil, err //nolint:wrapcheck // False-Positive.
	}

	return &webScraperResult{Code: code, Content: data}, nil
}

func (s *webScraperImpl) BuildQuery(target string, options func(map[string]string) map[string]string) string {
	conf := map[string]string{
		"render_js":  "1",
		"device":     "mobile",
		"proxy_type": "residential",
		"timeout":    "30000",
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

	censorer := &censorerImpl{
		Strings: []string{apiKey, apiURL},
	}

	return &webScraperImpl{
		ScrapeAPIURL: apiURL,
		APIKey:       apiKey,
		Fetcher:      &dataFetcherImpl{Censorer: censorer},
	}
}
