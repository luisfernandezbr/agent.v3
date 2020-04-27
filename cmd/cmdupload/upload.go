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

	"github.com/pinpt/agent/pkg/fs"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/archive"
	"github.com/pinpt/agent/pkg/fsconf"
	"github.com/pinpt/go-common/fileutil"
	"github.com/pinpt/go-common/upload"
)

var ErrNoFilesFound = errors.New("no files found to upload")

// Run uploads resulting export file.
// Pass path to logFile to include that in uploaded zip as well.
func Run(ctx context.Context,
	logger hclog.Logger,
	pinpointRoot string,
	uploadURL string,
	jobID string,
	apiKey string,
	logFile string) (parts int, size int64, rerr error) {

	fsc := fsconf.New(pinpointRoot)

	err := os.MkdirAll(fsc.UploadZips, 0777)
	if err != nil {
		rerr = err
		return
	}

	fileName := time.Now().Format(time.RFC3339)
	fileName = strings.ReplaceAll(fileName, ":", "_") + "-" + jobID

	zipPath := filepath.Join(fsc.UploadZips, fileName+".zip")

	logger.Info("looking for files", "dir", fsc.Uploads)
	files, err := fileutil.FindFiles(fsc.Uploads, regexp.MustCompile("\\.gz$"))
	if err != nil {
		rerr = err
		return
	}
	if len(files) == 0 {
		rerr = ErrNoFilesFound
		return
	}
	if logFile != "" {
		pathInUploads := filepath.Join(fsc.Uploads, "export.log")
		err := fs.CopyFile(logFile, pathInUploads)
		if err != nil {
			rerr = err
			return
		}
		files = append(files, pathInUploads)
	}

	err = archive.ZipFiles(zipPath, fsc.Uploads, files)
	if err != nil {
		rerr = err
		return
	}
	logger.Info("uploading export result", "upload_url", uploadURL, "zip_path", zipPath)

	parts, size, err = runUpload(logger, zipPath, uploadURL, apiKey)
	if err != nil {
		rerr = err
		return
	}

	logger.Info("zip file uploaded with no errors", "zip_path", zipPath, "size_kb", size/1024)
	if err = os.RemoveAll(zipPath); err != nil {
		rerr = fmt.Errorf("error deleting zip file %s", err)
		return
	}

	logger.Info("zip file deleted", "zip_path", zipPath)
	return
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
