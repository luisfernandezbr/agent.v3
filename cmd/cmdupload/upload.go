package cmdupload

import (
	"context"
	"errors"
	"fmt"
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
)

func Run(ctx context.Context,
	logger hclog.Logger,
	pinpointRoot string,
	uploadURL string,
	apiKey string) (parts int, size int64, err error) {

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

	parts, size, err = runUpload(logger, zipPath, uploadURL, apiKey)
	if err != nil {
		return
	}
	logger.Info("zip file uploaded with no errors", "zip_path", zipPath)

	if err = os.RemoveAll(zipPath); err != nil {
		err = fmt.Errorf("error deleting zip file %s", err)
		return
	}
	logger.Info("zip file deleted", "zip_path", zipPath)

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

func runUpload(logger hclog.Logger, zipPath, uploadURL, apiKey string) (parts int, uploadedSize int64, rerr error) {

	f, err := os.Open(zipPath)
	defer f.Close()
	if err != nil {
		rerr = err
		return
	}
	fi, err := f.Stat()
	if err != nil {
		rerr = err
		return
	}
	zipSize := fi.Size()

	parts, uploadedSize, err = upload.Upload(upload.Options{
		APIKey:      apiKey,
		Body:        f,
		ContentType: "application/zip",
		URL:         uploadURL,
	})

	if err != nil {
		rerr = err
		return
	}

	if uploadedSize != zipSize {
		rerr = fmt.Errorf("invalid updated size, zip: %v uploaded: %v", zipSize, uploadedSize)
		return
	}

	return
}
