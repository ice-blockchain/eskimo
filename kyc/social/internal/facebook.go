// SPDX-License-Identifier: ice License 1.0

package social

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/log"
)

const (
	scopePublicProfile = 1 << iota
	scopeUserPosts
	scopeEmail
	ScopeOpenID
)

func (f *facebookVerifierImpl) GenerateAppSecretProof(token string) string {
	hash := hmac.New(sha256.New, []byte(f.AppSecret))
	_, _ = hash.Write([]byte(token))

	return hex.EncodeToString(hash.Sum(nil))
}

func (f *facebookVerifierImpl) BuildURL(endpoint string, args map[string]string) string {
	const baseURL = "https://graph.facebook.com/"

	base, err := url.Parse(baseURL)
	if err != nil {
		log.Panic("invalid base URL: " + baseURL + ": " + err.Error())
	}

	query := base.Query()
	for k, v := range args {
		query.Set(k, v)
	}

	if v, ok := args["access_token"]; ok {
		query.Set("appsecret_proof", f.GenerateAppSecretProof(v))
	}

	base.RawQuery = query.Encode()

	return base.JoinPath(endpoint).String()
}

func (f *facebookVerifierImpl) VerifyUserFeed(ctx context.Context, meta *Metadata, expectedPostText string) error {
	const limitMax = 3
	targetURL := f.BuildURL("me/posts", map[string]string{"access_token": meta.AccessToken})
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
		data, err := f.Scraper.Scrape(ctx, targetURL, nil)
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

func (*facebookVerifierImpl) parseScopes(scopes []string) (result int) {
	for _, scope := range scopes {
		switch scope {
		case "public_profile":
			result |= scopePublicProfile
		case "user_posts":
			result |= scopeUserPosts
		case "email":
			result |= scopeEmail
		case "openid":
			result |= ScopeOpenID
		}
	}

	return result
}

func (f *facebookVerifierImpl) VerifyToken(ctx context.Context, meta *Metadata) (username string, err error) {
	var response facebookTokenResponse

	targetURL := f.BuildURL("debug_token",
		map[string]string{
			"input_token":  meta.AccessToken,
			"access_token": f.AppID + "|" + f.AppSecret,
		})

	data, err := f.Scraper.Scrape(ctx, targetURL, nil)
	if err != nil {
		return "", multierror.Append(ErrScrapeFailed, err)
	}

	err = json.Unmarshal(data, &response)
	if err != nil {
		return "", errors.Wrap(err, "failed to unmarshal response")
	}

	if !(response.Data.AppID == f.AppID && response.Data.Valid && response.Data.IssuedAt == 0) {
		return "", ErrInvalidToken
	} else if hasScopes, wantScopes := f.parseScopes(response.Data.Scopes), scopeUserPosts|scopePublicProfile; hasScopes&wantScopes != wantScopes {
		return "", errors.Wrap(ErrInvalidToken, "missing scopes")
	} else if response.Data.UserID == "" {
		return "", ErrUsernameNotFound
	}

	return response.Data.UserID, nil
}

func (f *facebookVerifierImpl) VerifyPost(ctx context.Context, meta *Metadata, _, expectedPostText string) (username string, err error) {
	username, err = f.VerifyToken(ctx, meta)
	if err != nil {
		return "", err
	}

	err = f.VerifyUserFeed(ctx, meta, expectedPostText)

	return username, err
}

func newFacebookVerifier(scraper webScraper, appID, appSecret string) *facebookVerifierImpl {
	if appID == "" || appSecret == "" || scraper == nil { //nolint:gosec // False-Positive.
		log.Panic("invalid Facebook config")
	}

	return &facebookVerifierImpl{
		AppID:     appID,
		AppSecret: appSecret,
		Scraper:   scraper,
	}
}
