package cmdbuild

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pinpt/agent/pkg/build"
	"github.com/pinpt/agent/pkg/fs"
)

func upload(opts Opts, platforms Platforms) {
	err := build.ValidateVersion(opts.Version)
	if err != nil {
		fmt.Println("invalid version", "err", err)
		os.Exit(1)
	}

	fmt.Println("Preparing upload, moving files")

	// prepare all files for upload
	releaseDir := fjoin(opts.BuildDir, "s3-release")

	err = os.RemoveAll(releaseDir)
	if err != nil {
		panic(err)
	}

	err = os.MkdirAll(releaseDir, 0777)
	if err != nil {
		panic(err)
	}

	// include unpacked agent binary unpacked for curl installer
	// TODO: this is temporary for testing, will be installing from github normally??
	copyAgentUnpackedIntoDir(opts, platforms, fjoin(releaseDir, "agent"), false)

	if opts.OnlyAgent {
		fmt.Println("only-agent passed skipping bin-gz folder upload, including gz agent")
	} else {
		err = fs.CopyDir(fjoin(opts.BuildDir, "bin-gz"), fjoin(releaseDir, "bin-gz"))
		if err != nil {
			panic(err)
		}
	}

	fmt.Println("Uploading to S3")

	awsSession := session.New(
		&aws.Config{
			Region: aws.String("us-east-1"),
		},
	)

	uploader := s3manager.NewUploader(awsSession)

	// if doing a new release that is not dev,
	// update the windows binary in root that is
	// used for installing latest version
	// do not update if OnlyPlatform is specified,
	// because it's not a full release in that case
	if opts.Version != "dev" && opts.OnlyPlatform == "" {
		src := fjoin(opts.BuildDir, "bin", "windows-amd64", "pinpoint-agent.exe")

		uploadFile(uploader,
			bucket,
			src,
			"pinpoint-agent.exe",
		)
	}

	// upload s3-release dir
	uploadDir(
		uploader,
		releaseDir,
		bucket,
		"releases/"+opts.Version)

}

const bucket = "pinpoint-agent"

func uploadDir(uploader *s3manager.Uploader, dir string, bucket string, prefix string) {
	walker := make(fileWalk)
	go func() {
		err := filepath.Walk(dir, walker.Walk)
		if err != nil {
			panic(err)
		}
		close(walker)
	}()
	for path := range walker {
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			panic(err)
		}
		uploadFile(uploader, bucket, path, filepath.Join(prefix, rel))
	}
}

func uploadFile(uploader *s3manager.Uploader, bucket string, localPath string, remoteKey string) {
	fmt.Println("Uploading", localPath)
	file, err := os.Open(localPath)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: &bucket,
		Key:    aws.String(remoteKey),
		Body:   file,
		ACL:    aws.String("public-read"),
	})
	if err != nil {
		fmt.Println("Upload failed")
		fmt.Println(err)
		os.Exit(1)
		//if strings.Contains(err.Error(), "expired") {
	}
	fmt.Println("Uploaded", localPath, result.Location)
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
