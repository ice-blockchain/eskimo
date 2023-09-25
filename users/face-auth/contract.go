// SPDX-License-Identifier: ice License 1.0

package faceauth

import (
	"context"
	"errors"
	stdlibtime "time"

	"github.com/imroc/req/v3"
)

// .
var (
	// .
	ErrNotAuthorized = errors.New("not authorized")
)

type (
	Client interface {
		DeleteUserFaces(ctx context.Context, userID, authToken, metadataToken string) error
	}
)

const (
	applicationYamlKey = "face-auth"
	deleteURL          = "/v1w/face-auth/"
	requestDeadline    = 30 * stdlibtime.Second
)

type (
	client struct {
		cfg        *config
		httpClient *req.Client
	}
	config struct {
		FaceAuthBaseURL string `yaml:"faceAuthBaseUrl"`
		APIKey          string
		CACertificates  []string `yaml:"caCertificates"`
	}
)
