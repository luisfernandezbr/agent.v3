package main

import (
	"io/ioutil"
	"net/http"
	"os"

	"github.com/hashicorp/go-hclog"
)

func Upload(logger hclog.Logger, zipPath, uploadURL string) error {
	logger.Info("uploading", "zip_path", zipPath, "upload_url", uploadURL)
	f, err := os.Open(zipPath)
	defer f.Close()
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, uploadURL, f)
	if err != nil {
		return err
	}
	req.ContentLength = fi.Size()
	req.Header.Set("Content-Type", "application/zip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	logger.Info("upload response", "data", string(data))

	return nil
}
