// SPDX-License-Identifier: ice License 1.0

package social

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strconv"
	"time"

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

func (f *facebookVerifierImpl) GenerateAppSecretProof(token string) (proof, timestamp string) {
	now := strconv.FormatInt(time.Now().Unix(), 10)
	hash := hmac.New(sha256.New, []byte(f.AppSecret))
	_, err := hash.Write([]byte(token + "|" + now))
	log.Panic(err)

	return hex.EncodeToString(hash.Sum(nil)), now
}

func (f *facebookVerifierImpl) BuildURL(endpoint string, args map[string]string) string {
	const baseURL = "https://graph.facebook.com/"

	base, err := url.Parse(baseURL)
	log.Panic(errors.Wrapf(err, "invalid base URL: %v ", baseURL)) //nolint:revive // False-Positive.

	query := base.Query()
	for k, v := range args {
		query.Set(k, v)
	}

	if v, ok := args["access_token"]; ok {
		proof, timestamp := f.GenerateAppSecretProof(v)
		query.Set("appsecret_proof", proof)
		query.Set("appsecret_time", timestamp)
	}

	base.RawQuery = query.Encode()

	return base.JoinPath(endpoint).String()
}

func (f *facebookVerifierImpl) FetchFeed(ctx context.Context, targetURL string) (resp facebookFeedResponse, err error) {
	data, err := f.Fetcher.Fetch(ctx, targetURL)
	if err != nil {
		return resp, multierror.Append(ErrScrapeFailed, err)
	}

	err = json.Unmarshal(data, &resp)
	if err != nil {
		return resp, errors.Wrap(err, "failed to unmarshal response")
	}

	return resp, nil
}

func (f *facebookVerifierImpl) ProcessFeedResponse(meta *Metadata, response *facebookFeedResponse) (err error) {
	err = ErrTextNotFound
	for entry := range response.Data {
		if response.Data[entry].Message != meta.ExpectedPostText {
			continue
		}

		for a := range response.Data[entry].Attachments.Data {
			if response.Data[entry].Attachments.Data[a].Type == "share" && response.Data[entry].Attachments.Data[a].URL == f.Post {
				return nil
			}
		}

		err = ErrPostNotFound
	}

	return err
}

func (f *facebookVerifierImpl) VerifyUserFeed(ctx context.Context, meta *Metadata, userID string) (err error) {
	const limitMax = 3
	hasPostWithoutRepost := false
	targetURL := f.BuildURL(userID+"/posts", map[string]string{
		"access_token": meta.AccessToken,
		"fields":       "attachments{type,unshimmed_url},message",
	})
	for limit := 0; (limit < limitMax) && (targetURL != ""); limit++ {
		var response facebookFeedResponse
		response, err = f.FetchFeed(ctx, targetURL)
		if err != nil {
			break
		}

		err = f.ProcessFeedResponse(meta, &response)
		if err == nil {
			break
		}

		hasPostWithoutRepost = hasPostWithoutRepost || errors.Is(err, ErrPostNotFound)
		targetURL = response.Paging.Previous
	}

	if hasPostWithoutRepost {
		err = ErrPostNotFound
	}

	return err
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

func (f *facebookVerifierImpl) VerifyToken(ctx context.Context, meta *Metadata) (userID string, err error) {
	var response facebookTokenResponse

	targetURL := f.BuildURL("debug_token",
		map[string]string{
			"input_token":  meta.AccessToken,
			"access_token": f.AppID + "|" + f.AppSecret,
		})

	data, err := f.Fetcher.Fetch(ctx, targetURL)
	if err != nil {
		return "", multierror.Append(ErrScrapeFailed, err)
	}

	err = json.Unmarshal(data, &response)
	if err != nil {
		return "", errors.Wrap(err, "failed to unmarshal response")
	}

	if !(response.Data.AppID == f.AppID && response.Data.Valid && response.Data.IssuedAt > 0) {
		return "", ErrInvalidToken
	} else if hasScopes, wantScopes := f.parseScopes(response.Data.Scopes), scopeUserPosts|scopePublicProfile; hasScopes&wantScopes != wantScopes {
		return "", errors.Wrap(ErrInvalidToken, "missing scopes")
	} else if response.Data.UserID == "" {
		return "", ErrUsernameNotFound
	}

	return response.Data.UserID, nil
}

func (f *facebookVerifierImpl) VerifyPost(ctx context.Context, meta *Metadata) (userID string, err error) {
	userID, err = f.VerifyToken(ctx, meta)
	if err != nil {
		return "", err
	}

	err = f.VerifyUserFeed(ctx, meta, userID)

	return userID, err
}

func newFacebookVerifier(fetcher dataFetcher, post, appID, appSecret string) *facebookVerifierImpl {
	if appID == "" || appSecret == "" || fetcher == nil { //nolint:gosec // False-Positive.
		log.Panic("invalid Facebook config")
	}

	return &facebookVerifierImpl{
		AppID:     appID,
		AppSecret: appSecret,
		Post:      post,
		Fetcher:   fetcher,
	}
}
