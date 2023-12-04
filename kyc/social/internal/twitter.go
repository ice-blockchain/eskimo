// SPDX-License-Identifier: ice License 1.0

package social

import (
	"bytes"
	"context"
	"encoding/json"
	"math/rand"
	"net/url"
	"slices"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/time"
)

func (*twitterVerifierImpl) VerifyText(doc *goquery.Document, expectedText string) (found bool) {
	doc.Find("p").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		found = found || strings.Contains(s.Text(), strings.TrimSpace(expectedText))

		return !found
	})

	return
}

func (t *twitterVerifierImpl) VerifyPostLink(ctx context.Context, doc *goquery.Document) (foundPost bool) {
	doc.Find("a").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		for _, node := range s.Nodes {
			for i := range node.Attr {
				if node.Attr[i].Key == "href" && strings.HasPrefix(node.Attr[i].Val, "https://t.co") {
					data, err := t.Scrape(ctx, node.Attr[i].Val)
					foundPost = err == nil && strings.Contains(string(data), strings.ToLower(t.Post))

					break
				}
			}
		}

		return !foundPost
	})

	return
}

func (t *twitterVerifierImpl) VerifyContent(ctx context.Context, oe *twitterOE, expectedText string) (err error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader([]byte(oe.HTML)))
	if err != nil {
		return multierror.Append(ErrInvalidPageContent, err)
	}

	if !t.VerifyText(doc, expectedText) {
		return ErrTextNotFound
	}

	if !t.VerifyPostLink(ctx, doc) {
		return ErrPostNotFound
	}

	return nil
}

func (*twitterVerifierImpl) ExtractUsernameFromURL(postURL string) (username string, err error) {
	const (
		expectedTokensLenMin  = 5
		expectedUsernameIndex = 3
		expectedStatusIndex   = 4
		expectedStatusText    = "status"
	)

	if tokens := strings.Split(postURL, "/"); len(tokens) > expectedTokensLenMin && //nolint:revive // False-Positive.
		tokens[expectedStatusIndex] == expectedStatusText {
		username = tokens[expectedUsernameIndex]
	}

	if username == "" {
		err = errors.Wrap(ErrUsernameNotFound, postURL)
	}

	return
}

func (t *twitterVerifierImpl) Scrape(ctx context.Context, target string) (content []byte, err error) {
	for _, country := range t.countries() {
		if content, err = t.Scraper.Scrape(ctx, target, func(m map[string]string) map[string]string {
			m["country"] = country
			delete(m, "render_js")
			delete(m, "wait_until")

			return m
		}); err == nil {
			break
		}
	}
	if err != nil {
		return nil, multierror.Append(ErrFetchFailed, err)
	}

	return content, nil
}

func (t *twitterVerifierImpl) FetchOE(ctx context.Context, postURL string) (*twitterOE, error) {
	var (
		data []byte
		err  error
	)

	target := url.URL{
		Scheme:   "https",
		Host:     "publish.twitter.com",
		Path:     "/oembed",
		RawQuery: url.Values{"url": {postURL}}.Encode(),
	}

	if data, err = t.Scrape(ctx, target.String()); err != nil {
		return nil, err
	}

	var oe twitterOE
	if err = json.Unmarshal(data, &oe); err != nil {
		return nil, multierror.Append(ErrInvalidPageContent, err)
	} else if oe.HTML == "" {
		return nil, errors.Wrap(ErrInvalidPageContent, "empty page")
	}

	return &oe, nil
}

func (t *twitterVerifierImpl) VerifyPost(ctx context.Context, meta *Metadata) (username string, err error) {
	validDomain := false
	for i := range t.Domains {
		validDomain = validDomain || hasRootDomainAndHTTPS(meta.PostURL, t.Domains[i])
	}
	if !validDomain {
		return "", errors.Wrap(ErrInvalidURL, meta.PostURL)
	}

	username, err = t.ExtractUsernameFromURL(meta.PostURL)
	if username == "" {
		return "", err
	}

	oe, err := t.FetchOE(ctx, meta.PostURL)
	if err != nil {
		return "", err
	}

	return username, t.VerifyContent(ctx, oe, meta.ExpectedPostText)
}

func (t *twitterVerifierImpl) countries() []string {
	countries := slices.Clone(t.Countries)
	rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(countries), func(ii, jj int) { //nolint:gosec // .
		countries[ii], countries[jj] = countries[jj], countries[ii]
	})

	return countries
}

func newTwitterVerifier(sc webScraper, post string, allowedDomains, countries []string) *twitterVerifierImpl {
	return &twitterVerifierImpl{
		Scraper:   sc,
		Post:      post,
		Domains:   allowedDomains,
		Countries: countries,
	}
}
