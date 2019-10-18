package cmdupload

import (
	"context"
	"errors"
	"fmt"
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
	uploadURL string) (size int64, err error) {

	fsc := fsconf.New(pinpointRoot)

	err = os.MkdirAll(fsc.UploadZips, 0777)
	if err != nil {
		return
	}

	fileName := time.Now().Format(time.RFC3339)
	fileName = strings.ReplaceAll(fileName, ":", "_")

	zipPath := filepath.Join(fsc.UploadZips, fileName+".zip")

	err = zipFilesJSON(logger, zipPath, fsc.Uploads)
	if err != nil {
		return
	}
	logger.Info("uploading export result", "upload_url", uploadURL, "zip_path", zipPath)

	size, err = upload(logger, zipPath, uploadURL)
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

func upload(logger hclog.Logger, zipPath, uploadURL string) (size int64, err error) {

	f, err := os.Open(zipPath)
	defer f.Close()
	if err != nil {
		return 0, err
	}
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodPut, uploadURL, f)
	if err != nil {
		return 0, err
	}
	size = fi.Size()
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/zip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		io.Copy(ioutil.Discard, resp.Body) // copy even if we don't read
		logger.Info("Upload completed without error")
		return size, nil
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	logger.Error("Upload failed", "response_status", resp.StatusCode, "response", string(data))
	return 0, fmt.Errorf("upload failed with server status code: %v", resp.StatusCode)
}
