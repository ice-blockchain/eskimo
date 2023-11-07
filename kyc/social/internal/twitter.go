// SPDX-License-Identifier: ice License 1.0

package social

import (
	"bytes"
	"context"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

func (*twitterVerifierImpl) VerifyText(doc *goquery.Document, expectedText string) (found bool) {
	doc.Find("title").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		text := s.Text()

		idx := strings.Index(text, expectedText)
		if idx > 0 {
			text = text[idx:]
			if end := strings.IndexByte(text, '\n'); end > 0 {
				text = text[:end]
			}
			text = strings.TrimSpace(text)
			found = found || text == expectedText
		}

		return !found
	})

	return
}

func (t *twitterVerifierImpl) VerifyPostLink(doc *goquery.Document) (foundPost bool) {
	doc.Find("*").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		for _, node := range s.Nodes {
			for i := range node.Attr {
				if node.Attr[i].Key == "href" && node.Attr[i].Val == t.Post {
					foundPost = true

					break
				}
			}
		}

		return !foundPost
	})

	return
}

func (t *twitterVerifierImpl) VerifyContent(content []byte, expectedText string) (err error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
	if err != nil {
		return multierror.Append(ErrInvalidPageContent, err)
	}

	if !t.VerifyText(doc, expectedText) {
		return ErrTextNotFound
	}

	if !t.VerifyPostLink(doc) {
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

	data, err := t.Scraper.Scrape(ctx, meta.PostURL, nil)
	if err != nil {
		return "", multierror.Append(ErrFetchFailed, err)
	}

	return username, t.VerifyContent(data, meta.ExpectedPostText)
}

func newTwitterVerifier(sc webScraper, post string, allowedDomains []string) *twitterVerifierImpl {
	return &twitterVerifierImpl{
		Scraper: sc,
		Post:    post,
		Domains: allowedDomains,
	}
}
