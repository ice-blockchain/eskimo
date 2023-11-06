// SPDX-License-Identifier: ice License 1.0

package social

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/log"
)

func (*facebookVerifierImpl) BuildURL(endpoint string, meta *Metadata) string {
	const baseURL = "https://graph.facebook.com/"

	base, err := url.Parse(baseURL)
	if err != nil {
		log.Panic("invalid base URL: " + baseURL + ": " + err.Error())
	}

	q := base.Query()
	q.Set("access_token", meta.AccessToken)
	base.RawQuery = q.Encode()

	return base.JoinPath(endpoint).String()
}

func (*facebookVerifierImpl) setupScraperOpts(map[string]string) map[string]string {
	return map[string]string{
		"country": "US",
	}
}

func (f *facebookVerifierImpl) FetchUserHandle(ctx context.Context, target string) (string, error) {
	var selfInfo struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}

	data, err := f.Scraper.Scrape(ctx, target, f.setupScraperOpts)
	if err != nil {
		return "", multierror.Append(ErrScrapeFailed, err)
	}

	err = json.Unmarshal(data, &selfInfo)
	if err != nil {
		return "", errors.Wrap(err, "failed to unmarshal response")
	} else if selfInfo.ID == "" {
		return "", ErrUsernameNotFound
	}

	return selfInfo.ID, nil
}

func (f *facebookVerifierImpl) VerifyUserFeed(ctx context.Context, target, expectedPostText string) error {
	const limitMax = 3
	targetURL := target
	for limit := 0; (limit < limitMax) && (targetURL != ""); limit++ {
		var response struct {
			Paging struct {
				Next     string `json:"next"`
				Previous string `json:"previous"`
			} `json:"paging"`
			Data []struct {
				Message string `json:"message"`
				ID      string `json:"id"`
			} `json:"data"`
		}
		data, err := f.Scraper.Scrape(ctx, targetURL, f.setupScraperOpts)
		if err != nil {
			return multierror.Append(ErrScrapeFailed, err)
		}
		err = json.Unmarshal(data, &response)
		if err != nil {
			return errors.Wrap(err, "failed to unmarshal response")
		}
		for i := range response.Data {
			if response.Data[i].Message == expectedPostText {
				return nil
			}
		}
		targetURL = response.Paging.Previous
	}

	return ErrTextNotFound
}

func (f *facebookVerifierImpl) VerifyPost(ctx context.Context, meta *Metadata, _, expectedPostText string) (username string, err error) {
	username, err = f.FetchUserHandle(ctx, f.BuildURL("me", meta))
	if err != nil {
		return "", err
	}

	err = f.VerifyUserFeed(ctx, f.BuildURL("me/posts", meta), expectedPostText)

	return username, err
}

func newFacebookVerifier(sc webScraper) *facebookVerifierImpl {
	return &facebookVerifierImpl{
		Scraper: sc,
	}
}
