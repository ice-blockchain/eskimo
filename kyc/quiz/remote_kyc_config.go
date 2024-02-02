// SPDX-License-Identifier: ice License 1.0

package quiz

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	stdlibtime "time"

	"github.com/goccy/go-json"
	"github.com/imroc/req/v3"
	"github.com/pkg/errors"

	"github.com/ice-blockchain/wintr/log"
)

func init() { //nolint:gochecknoinits // It's the only way to tweak the client.
	req.DefaultClient().SetJsonMarshal(json.Marshal)
	req.DefaultClient().SetJsonUnmarshal(json.Unmarshal)
	req.DefaultClient().GetClient().Timeout = requestDeadline
}

func (r *repositoryImpl) startKYCConfigJSONSyncer(ctx context.Context) {
	ticker := stdlibtime.NewTicker(stdlibtime.Minute)
	defer ticker.Stop()
	r.config.kycConfigJSON = new(atomic.Pointer[kycConfigJSON])
	log.Panic(errors.Wrap(r.syncKYCConfigJSON(ctx), "failed to syncKYCConfigJSON")) //nolint:revive // .

	for {
		select {
		case <-ticker.C:
			reqCtx, cancel := context.WithTimeout(ctx, requestDeadline)
			log.Error(errors.Wrap(r.syncKYCConfigJSON(reqCtx), "failed to syncKYCConfigJSON"))
			cancel()
		case <-ctx.Done():
			return
		}
	}
}

//nolint:funlen,gomnd // .
func (r *repositoryImpl) syncKYCConfigJSON(ctx context.Context) error {
	if resp, err := req.
		SetContext(ctx).
		SetRetryCount(25).
		SetRetryBackoffInterval(10*stdlibtime.Millisecond, 1*stdlibtime.Second).
		SetRetryHook(func(resp *req.Response, err error) {
			if err != nil {
				log.Error(errors.Wrap(err, "failed to fetch KYCConfigJSON, retrying...")) //nolint:revive // .
			} else {
				log.Error(errors.Errorf("failed to fetch KYCConfigJSON with status code:%v, retrying...", resp.GetStatusCode())) //nolint:revive // .
			}
		}).
		SetRetryCondition(func(resp *req.Response, err error) bool {
			return err != nil || resp.GetStatusCode() != http.StatusOK
		}).
		SetHeader("Accept", "application/json").
		SetHeader("Cache-Control", "no-cache, no-store, must-revalidate").
		SetHeader("Pragma", "no-cache").
		SetHeader("Expires", "0").
		Get(r.config.ConfigJSONURL); err != nil {
		return errors.Wrapf(err, "failed to get fetch `%v`", r.config.ConfigJSONURL)
	} else if data, err2 := resp.ToBytes(); err2 != nil {
		return errors.Wrapf(err2, "failed to read body of `%v`", r.config.ConfigJSONURL)
	} else { //nolint:revive // .
		var kycConfig kycConfigJSON
		if err = json.UnmarshalContext(ctx, data, &kycConfig); err != nil {
			return errors.Wrapf(err, "failed to unmarshal into %#v, data: %v", kycConfig, string(data))
		}
		if body := string(data); !strings.Contains(body, "face-auth") && !strings.Contains(body, "web-face-auth") {
			return errors.Errorf("there's something wrong with the KYCConfigJSON body: %v", body)
		}
		r.config.kycConfigJSON.Swap(&kycConfig)

		return nil
	}
}

func (r *repositoryImpl) isKYCEnabled(ctx context.Context) bool {
	var (
		kycConfig = r.config.kycConfigJSON.Load()
		isWeb     = isWebClientType(ctx)
	)

	if isWeb && !kycConfig.WebQuizKYC.Enabled {
		return false
	}
	if !isWeb && !kycConfig.QuizKYC.Enabled {
		return false
	}
	if !isWeb && kycConfig.QuizKYC.Enabled {
		return false
	}

	return true
}

func ContextWithClientType(ctx context.Context, clientType string) context.Context {
	if clientType == "" {
		return ctx
	}

	return context.WithValue(ctx, clientTypeCtxValueKey, clientType) //nolint:revive,staticcheck // Not an issue.
}

func isWebClientType(ctx context.Context) bool {
	clientType, _ := ctx.Value(clientTypeCtxValueKey).(string) //nolint:errcheck,revive // Not needed.

	return strings.EqualFold(strings.TrimSpace(clientType), "web")
}
