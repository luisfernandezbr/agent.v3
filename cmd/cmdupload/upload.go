package cmdupload

import (
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pinpt/go-common/fileutil"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/archive"
	"github.com/pinpt/agent.next/pkg/fsconf"
)

func Run(ctx context.Context,
	logger hclog.Logger,
	pinpointRoot string,
	uploadURL string) error {

	fsc := fsconf.New(pinpointRoot)

	err := os.MkdirAll(fsc.Temp, 0777)
	if err != nil {
		return err
	}

	dir, err := ioutil.TempDir(fsc.Temp, "upload")
	if err != nil {
		return err
	}

	zipPath := filepath.Join(dir, "upload.zip")

	err = zipFilesJSON(logger, zipPath, fsc.Uploads)
	if err != nil {
		return err
	}

	err = upload(logger, zipPath, uploadURL)
	if err != nil {
		return err
	}

	return nil
}

func zipFilesJSON(logger hclog.Logger, target, source string) error {
	logger.Info("looking for files", "dir", source)
	files, err := fileutil.FindFiles(source, regexp.MustCompile("\\.gz$"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no files to upload")
	}
	return archive.ZipFiles(target, source, files)
}

func upload(logger hclog.Logger, zipPath, uploadURL string) error {
	logger.Info("uploading", "zip_path", zipPath)
	f, err := os.Open(zipPath)
	defer f.Close()
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PUT", uploadURL, f)
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

	/*
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		logger.Info("upload response", "data", string(data))
	*/

	return nil
}
