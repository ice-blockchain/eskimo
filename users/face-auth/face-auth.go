// SPDX-License-Identifier: ice License 1.0

package faceauth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	stdlibtime "time"

	"github.com/cenkalti/backoff/v4"
	"github.com/hashicorp/go-multierror"
	"github.com/imroc/req/v3"
	"github.com/pkg/errors"

	appCfg "github.com/ice-blockchain/wintr/config"
	"github.com/ice-blockchain/wintr/log"
	"github.com/ice-blockchain/wintr/terror"
)

func NewClient(apiKey string) Client {
	var cfg config
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)
	cfg.APIKey = apiKey
	httpClient := req.C().SetBaseURL(cfg.FaceAuthBaseURL)
	if len(cfg.CACertificates) > 0 {
		caCertPool := x509.NewCertPool()
		for _, crt := range cfg.CACertificates {
			caCertPool.AppendCertsFromPEM([]byte(crt))
		}
		httpClient = httpClient.SetTLSClientConfig(&tls.Config{RootCAs: caCertPool}) //nolint:gosec // .
	}

	return &client{cfg: &cfg, httpClient: httpClient}
}

func (c *client) DeleteUserFaces(ctx context.Context, userID, authToken, metadataToken string) error {
	if c.cfg.FaceAuthBaseURL == "" {
		return nil
	}

	return errors.Wrapf(backoff.RetryNotify(
		c.deleteUserFace(userID, authToken, metadataToken),
		//nolint:gomnd // Because those are static configs.
		backoff.WithContext(&backoff.ExponentialBackOff{
			InitialInterval:     100 * stdlibtime.Millisecond,
			RandomizationFactor: 0.5,
			Multiplier:          2.5,
			MaxInterval:         stdlibtime.Second,
			MaxElapsedTime:      requestDeadline,
			Stop:                backoff.Stop,
			Clock:               backoff.SystemClock,
		}, ctx),
		func(e error, next stdlibtime.Duration) {
			log.Error(errors.Wrapf(e, "deleteFace for userID %v failed. retrying in %v... ", userID, next))
		}), "failed to delete users faces, userID %v", userID)
}

func (c *client) deleteUserFace(userID, authToken, metadataToken string) func() error {
	return func() error {
		response, err := c.httpClient.R().
			SetHeader("X-API-Key", c.cfg.APIKey).
			SetHeader("Authorization", fmt.Sprintf("Bearer %v", authToken)).
			SetHeader("X-Account-Metadata", metadataToken).
			Delete(deleteURL)
		if err != nil {
			return errors.Wrapf(err, "call for user's faces deletion failed, userID %v", userID)
		}
		if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusNoContent {
			body, bErr := response.ToString()
			if response.StatusCode == http.StatusUnauthorized {
				return terror.New(ErrNotAuthorized, map[string]any{"body": body})
			}

			return multierror.Append( //nolint:wrapcheck //.
				errors.Errorf("call user's faces deletion failed with %v %v userID %v", response.StatusCode, body, userID),
				bErr,
			).ErrorOrNil()
		}

		return nil
	}
}
