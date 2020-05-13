package cmdbuild

import (
	"fmt"

	"github.com/pinpt/go-common/fileutil"
)

type Opts struct {
	BuildDir     string
	Version      string // version to use in s3 upload
	Upload       bool   // set to true to upload to s3
	OnlyUpload   bool   // set to true to skip build and upload existing files in dist dir. Also sets upload to true.
	OnlyPlatform string // only build for this platform
	OnlyAgent    bool   // build only agent and skip the rest
	SkipArchives bool   // do not create zips and gzips
}

var integrationBinaries = []string{
	"azure",
	"bitbucket",
	"github",
	"gitlab",
	"jira-cloud",
	"jira-hosted",
	"mock",
	"sonarqube",
	"gcal",
	"office365",
}

type Platform struct {
	UserFriedlyName string
	GOOS            string
	GOARCH          string
	BinSuffix       string
}

func (s Platform) OSArch() string {
	return s.GOOS + "-" + s.GOARCH
}

func Run(opts Opts) {
	platforms := getPlatforms(opts.OnlyPlatform)
	if len(platforms) == 0 {
		panic("passed platform is not valid: " + opts.OnlyPlatform)
	}

	if fileutil.FileExists(opts.BuildDir) && opts.OnlyUpload {
		fmt.Println("Skipping build ./dist directory exists")
	} else {
		doBuild(opts, platforms)
	}

	if opts.Upload || opts.OnlyUpload {
		upload(opts, platforms)
	}

	fmt.Println("All done!")
}

func getPlatforms(want string) []Platform {
	allPlatforms := []Platform{
		{UserFriedlyName: "linux", GOOS: "linux", GOARCH: "amd64"},
		{UserFriedlyName: "windows", GOOS: "windows", GOARCH: "amd64", BinSuffix: ".exe"},
	}
	if want == "" {
		return allPlatforms
	}
	allPlatforms = append(allPlatforms,
		Platform{UserFriedlyName: "macos", GOOS: "darwin", GOARCH: "amd64"})
	for _, pl := range allPlatforms {
		if pl.UserFriedlyName == want || pl.GOOS == want {
			return []Platform{pl}
		}
	}
	return nil
}
