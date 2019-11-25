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
}

var integrationBinaries = []string{
	"azuretfs",
	"bitbucket",
	"github",
	"gitlab",
	"jira-cloud",
	"jira-hosted",
	"mock",
	"sonarqube",
}

type Platform struct {
	Name string
	GOOS string
}

func (s Platform) String() string {
	return s.Name
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
		upload(opts)
	}

	fmt.Println("All done!")
}

func getPlatforms(want string) []Platform {
	allPlatforms := []Platform{
		{Name: "macos", GOOS: "darwin"},
		{Name: "linux", GOOS: "linux"},
		{Name: "windows", GOOS: "windows"},
	}
	if want == "" {
		return allPlatforms
	}
	for _, pl := range allPlatforms {
		if pl.Name == want || pl.GOOS == want {
			return []Platform{pl}
		}
	}
	return nil
}
