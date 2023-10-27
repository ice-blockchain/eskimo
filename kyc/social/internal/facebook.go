// SPDX-License-Identifier: ice License 1.0

package social

import (
	"context"
)

func (*facebookVerifierImpl) VerifyPost(context.Context, *Metadata, string, string) (string, error) {
	return "", ErrUnavailable
}

func newFacebookVerifier(sc webScraper) *facebookVerifierImpl {
	return &facebookVerifierImpl{
		Scraper: sc,
	}
}
