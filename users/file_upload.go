package users

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/imroc/req/v3"
	"github.com/pkg/errors"

	appCfg "github.com/ICE-Blockchain/wintr/config"
)

func RetryUpload(ctx context.Context, data *multipart.FileHeader) error {
	operation := func() error {
		return UploadProfilePicture(data)
	}
	//nolint:wrapcheck // No need, its just a proxy.
	return backoff.Retry(
		operation,
		//nolint:gomnd // Because those are static configs.
		backoff.WithContext(&backoff.ExponentialBackOff{
			InitialInterval:     100 * time.Millisecond,
			RandomizationFactor: 0.5,
			Multiplier:          2.5,
			MaxInterval:         time.Second,
			MaxElapsedTime:      25 * time.Second,
			Stop:                backoff.Stop,
			Clock:               backoff.SystemClock,
		}, ctx))
}

func UploadProfilePicture(data *multipart.FileHeader) error {
	appCfg.MustLoadFromKey(applicationYamlKey, &cfg)
	file, err := data.Open()
	defer file.Close()

	if err != nil {
		return errors.Wrap(err, "error opening file")
	}

	fileData, err := io.ReadAll(file)
	if err != nil {
		return errors.Wrap(err, "error reading file")
	}

	url := fmt.Sprintf("%s/%s/%s/%s", cfg.Storage.URL,
		cfg.Storage.ZoneName,
		cfg.Storage.ProfilePath,
		data.Filename)

	resp, err := req.
		SetHeader("AccessKey", cfg.Storage.AccessKey).
		SetHeader("Content-Type", data.Header.Get("Content-Type")).
		SetBodyBytes(fileData).Put(url)

	if err != nil && resp.StatusCode == http.StatusTooManyRequests {
		return errors.Wrap(err, "error uploading file")
	}

	return nil
}
