package cmdbuild

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pinpt/agent.next/pkg/build"
)

func upload(opts Opts) {
	err := build.ValidateVersion(opts.Version)
	if err != nil {
		fmt.Println("invalid version", "err", err)
		os.Exit(1)
	}

	fmt.Println("Uploading to S3")
	awsSession := session.New(
		&aws.Config{
			Region: aws.String("us-east-1"),
		},
	)
	uploadDir(
		awsSession,
		fjoin(opts.BuildDir, "bin"),
		"pinpoint-agent",
		"releases/"+opts.Version)
}

func uploadDir(awsSession *session.Session, localPath string, bucket string, prefix string) {
	walker := make(fileWalk)
	go func() {
		err := filepath.Walk(localPath, walker.Walk)
		if err != nil {
			panic(err)
		}
		close(walker)
	}()
	uploader := s3manager.NewUploader(awsSession)
	for path := range walker {
		rel, err := filepath.Rel(localPath, path)
		if err != nil {
			panic(err)
		}
		file, err := os.Open(path)
		if err != nil {
			panic(err)
		}
		defer file.Close()
		fmt.Println("Uploading", path)
		result, err := uploader.Upload(&s3manager.UploadInput{
			Bucket: &bucket,
			Key:    aws.String(filepath.Join(prefix, rel)),
			Body:   file,
		})
		if err != nil {
			fmt.Println("Upload failed")
			fmt.Println(err)
			os.Exit(1)
			//if strings.Contains(err.Error(), "expired") {
		}
		fmt.Println("Uploaded", path, result.Location)
	}
}

type fileWalk chan string

func (f fileWalk) Walk(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if info.Name() == ".DS_Store" {
		return nil
	}
	if info.IsDir() {
		return nil
	}
	f <- path
	return nil
}
