package cmdupload

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/archive"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/go-common/fileutil"
	"github.com/pinpt/go-common/upload"
	"github.com/pinpt/integration-sdk/agent"
)

func Run(ctx context.Context,
	logger hclog.Logger,
	pinpointRoot string,
	data *agent.ExportRequest) (parts int, size int64, err error) {

	fsc := fsconf.New(pinpointRoot)

	err = os.MkdirAll(fsc.UploadZips, 0777)
	if err != nil {
		return
	}

	uploadURL := *data.UploadURL
	fileName := time.Now().Format(time.RFC3339)
	fileName = strings.ReplaceAll(fileName, ":", "_")

	zipPath := filepath.Join(fsc.UploadZips, fileName+".zip")

	err = zipFilesJSON(logger, zipPath, fsc.Uploads)
	if err != nil {
		return
	}
	logger.Info("uploading export result", "upload_url", uploadURL, "zip_path", zipPath)

	parts, size, err = runUpload(logger, zipPath, uploadURL)
	if err != nil {
		return
	}

	return
}

var ErrNoFilesFound = errors.New("no files found to upload")

func zipFilesJSON(logger hclog.Logger, target, source string) error {
	logger.Info("looking for files", "dir", source)
	files, err := fileutil.FindFiles(source, regexp.MustCompile("\\.gz$"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return ErrNoFilesFound
	}
	return archive.ZipFiles(target, source, files)
}

func runUpload(logger hclog.Logger, zipPath, uploadURL string) (parts int, size int64, err error) {

	f, err := os.Open(zipPath)
	defer f.Close()
	if err != nil {
		return 0, 0, err
	}

	parts, size, err = upload.Upload(upload.Options{
		Body:        f,
		ContentType: "application/zip",
		URL:         uploadURL,
	})

	return
}
