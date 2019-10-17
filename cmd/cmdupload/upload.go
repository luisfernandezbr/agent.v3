package cmdupload

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pinpt/go-common/fileutil"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/archive"
	"github.com/pinpt/agent.next/pkg/fsconf"
)

func Run(ctx context.Context,
	logger hclog.Logger,
	pinpointRoot string,
	uploadURL string) (zipPath string, err error) {

	fsc := fsconf.New(pinpointRoot)

	err = os.MkdirAll(fsc.UploadZips, 0777)
	if err != nil {
		return
	}

	fileName := time.Now().Format(time.RFC3339)
	fileName = strings.ReplaceAll(fileName, ":", "_")

	zipPath = filepath.Join(fsc.UploadZips, fileName+".zip")

	err = zipFilesJSON(logger, zipPath, fsc.Uploads)
	if err != nil {
		return
	}
	logger.Info("uploading export result", "upload_url", uploadURL, "zip_path", zipPath)

	err = upload(logger, zipPath, uploadURL)
	if err != nil {
		return
	}

	return
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
	io.Copy(ioutil.Discard, resp.Body) // copy even if we don't read

	/*
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		logger.Info("upload response", "data", string(data))
	*/

	return nil
}
