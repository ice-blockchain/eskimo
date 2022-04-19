package users

import (
	"context"
	"fmt"
	appCfg "github.com/ICE-Blockchain/wintr/config"
	"github.com/imroc/req/v3"
	"github.com/pkg/errors"
	"io"
	"mime/multipart"
)

func (u *users) UploadProfilePicture(_ context.Context, data *multipart.FileHeader) error {
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

	fmt.Println(url)

	_, err = req.
		SetHeader("AccessKey", cfg.Storage.AccessKey).
		SetHeader("Content-Type", data.Header.Get("Content-Type")).
		SetBodyBytes(fileData).
		Put(url)

	if err != nil {
		return errors.Wrap(err, "error uploading file")
	}

	return nil
}
