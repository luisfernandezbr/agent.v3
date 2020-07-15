package cmddownloadexports

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pinpt/agent/pkg/archive"

	"github.com/pinpt/agent/pkg/fs"

	pstrings "github.com/pinpt/go-common/v10/strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type Opts struct {
	Channel    string
	CustomerID string
	OutputDir  string
}

func Run(opts Opts) {
	if opts.Channel == "" || opts.CustomerID == "" || opts.OutputDir == "" {
		panic("provide all required params")
	}

	downloadArchives(opts)
	unpackAndMerge(opts)
}

func downloadArchives(opts Opts) {
	fmt.Println("downloading archives")

	archivesDir := filepath.Join(opts.OutputDir, "archives")
	doneFile := filepath.Join(archivesDir, "done")
	exists, err := fs.Exists(doneFile)
	if err != nil {
		panic(err)
	}
	if exists {
		fmt.Println("skipping downloading archvices, since done file exists")
		return
	}

	awsSession := session.New(
		&aws.Config{
			Region: aws.String("us-east-1"),
		},
	)

	s3Client := s3.New(awsSession)

	bucket := getBucket(opts.Channel)

	params := &s3.ListObjectsInput{
		Bucket: pstrings.Pointer(bucket),
		Prefix: pstrings.Pointer("pinpt/" + opts.CustomerID),
	}

	var keys []string
	dirs := map[string]bool{}
	dirsWithZips := map[string]bool{}

	err = s3Client.ListObjectsPages(params, func(page *s3.ListObjectsOutput, more bool) bool {
		for _, obj := range page.Contents {
			k := *obj.Key
			dirs[path.Dir(k)] = true
			if !strings.HasSuffix(k, ".zip") {
				continue
			}
			dirsWithZips[path.Dir(k)] = true
			keys = append(keys, *obj.Key)
		}
		return true
	})
	if err != nil {
		panic(err)
	}

	warnMergedExports(dirs, dirsWithZips)

	downloader := s3manager.NewDownloader(awsSession)

	for _, key := range keys {
		target := filepath.Join(archivesDir, key)
		downloadFile(downloader, bucket, key, target)
	}

	err = ioutil.WriteFile(doneFile, nil, 0777)
	if err != nil {
		panic(err)
	}

	warnMergedExports(dirs, dirsWithZips)
}

func warnMergedExports(dirs, dirsWithZips map[string]bool) {
	for d := range dirs {
		if !dirsWithZips[d] {
			fmt.Printf("Error! Export dir does not have merged zip! %v\n", d)
		}
	}
}

func getBucket(channel string) string {
	switch channel {
	case "stable":
		return "pinpoint-stable-batch-bucket"
	case "edge":
		return "pinpoint-edge-batch-bucket"
	default:
		panic("unknown channel")
	}
}

func downloadFile(downloader *s3manager.Downloader, bucket string, key string, target string) {
	err := os.MkdirAll(filepath.Dir(target), 0777)
	if err != nil {
		panic(err)
	}
	f, err := os.Create(target)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	fmt.Println("downloading", key)
	params := &s3.GetObjectInput{Bucket: &bucket, Key: &key}
	_, err = downloader.Download(f, params)
	if err != nil {
		panic(err)
	}
}

func unpackAndMerge(opts Opts) {
	fmt.Println("unpacking and merging archives")
	archivesDir := filepath.Join(opts.OutputDir, "archives")
	targetDir := filepath.Join(opts.OutputDir, "merged")
	err := os.RemoveAll(targetDir)
	if err != nil {
		panic(err)
	}
	var archives []string
	err = filepath.Walk(archivesDir, func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".zip") {
			archives = append(archives, path)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	for _, arc := range archives {
		err := archive.Unzip(targetDir, arc)
		if err != nil {
			panic(err)
		}
	}
	fmt.Println("done unpacking and merging archives")
}
