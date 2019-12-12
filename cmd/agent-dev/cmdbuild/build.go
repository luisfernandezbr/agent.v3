package cmdbuild

import (
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/pinpt/agent.next/pkg/archive"
	"github.com/pinpt/agent.next/pkg/fs"
)

func doBuild(opts Opts, platforms Platforms) {
	err := os.RemoveAll(opts.BuildDir)
	if err != nil {
		panic(err)
	}

	fmt.Println("Building for platforms", platforms)

	{
		// create a agent binary
		commitSHA := getCommitSHA()
		fmt.Println("Repo commit sha", commitSHA)
		pkg := "github.com/pinpt/agent.next"
		ldflags := "-X " + pkg + "/cmd.Commit=" + commitSHA
		ldflags += " -X " + pkg + "/cmd.Version=" + opts.Version
		ldflags += " -X " + pkg + "/cmd.IntegrationBinariesAll=" + strings.Join(integrationBinaries, ",")

		platforms.Each(func(pl Platform) {
			buildAgent(opts, pl, ldflags)
		})
	}

	if opts.OnlyAgent {
		fmt.Println("only-agent passed, skipping rest")
		return
	}

	{
		// build integrations
		concurrency := runtime.NumCPU() * 2 / len(platforms)
		inChan := stringsToChan(integrationBinaries)
		wg := sync.WaitGroup{}
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for in := range inChan {
					platforms.Each(func(pl Platform) {
						buildIntegration(opts, in, pl)
					})
				}
			}()
		}
		wg.Wait()
	}

	if opts.SkipArchives {
		fmt.Println("skip archives, skipping creating zips and gzips")
		return
	}

	{
		// create archives
		err := os.MkdirAll(fjoin(opts.BuildDir, "archives"), 0777)
		if err != nil {
			panic(err)
		}

		fmt.Println("creating archives", platforms)
		platforms.Each(func(pl Platform) {
			err := archive.ZipDir(fjoin(opts.BuildDir, "archives", pl.Name+".zip"), fjoin(opts.BuildDir, "bin", pl.Name))
			if err != nil {
				panic(err)
			}
		})
	}

	gzipAgentAndIntegrations(opts, platforms)
	prepareGithubReleaseFiles(opts, platforms)
}

func gzipAgentAndIntegrations(opts Opts, platforms Platforms) {
	fmt.Println("creating gzipped binaries for auto-updater", platforms)

	platforms.Each(func(pl Platform) {
		nameInBin := fjoin(pl.Name, "pinpoint-agent")
		if pl.GOOS == "windows" {
			nameInBin += ".exe"
		}
		gzipBin(opts, nameInBin)
		integrationsDir := fjoin(opts.BuildDir, "bin", pl.Name, "integrations")
		files, err := ioutil.ReadDir(integrationsDir)
		if err != nil {
			panic(err)
		}
		for _, file := range files {
			nameInBin := fjoin(pl.Name, "integrations", file.Name())
			gzipBin(opts, nameInBin)
		}
	})
}

func gzipBin(opts Opts, nameInBin string) {
	srcLoc := fjoin(opts.BuildDir, "bin", nameInBin)
	trgLoc := fjoin(opts.BuildDir, "bin-gz", nameInBin+".gz")

	err := os.MkdirAll(filepath.Dir(trgLoc), 0777)
	if err != nil {
		panic(err)
	}
	r, err := os.Open(srcLoc)
	if err != nil {
		panic(err)
	}
	defer r.Close()
	f, err := os.Create(trgLoc)
	if err != nil {
		panic(err)
	}
	wr, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		panic(err)
	}
	_, err = io.Copy(wr, r)
	if err != nil {
		panic(err)
	}
	err = wr.Close()
	if err != nil {
		panic(err)
	}
	err = f.Close()
	if err != nil {
		panic(err)
	}
}

type Platforms []Platform

func (s Platforms) String() string {
	res := []string{}
	for _, pl := range s {
		res = append(res, pl.String())
	}
	return strings.Join(res, ",")
}

func prepareGithubReleaseFiles(opts Opts, platforms Platforms) {
	fmt.Println("copy files for github release", platforms)

	releaseDir := fjoin(opts.BuildDir, "github-release")
	err := os.MkdirAll(releaseDir, 0777)
	if err != nil {
		panic(err)
	}

	// include zip archives
	dir := fjoin(opts.BuildDir, "archives")
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}
	for _, f := range files {
		n := f.Name()
		err := fs.CopyFile(fjoin(dir, n), fjoin(releaseDir, "agent-with-integrations-"+n))
		if err != nil {
			panic(err)
		}
	}
	// include unpacked agent binary unpacked for curl installer
	copyAgentUnpackedIntoDir(opts, platforms, releaseDir)
}

func copyAgentUnpackedIntoDir(opts Opts, platforms Platforms, targetDir string) {

	err := os.MkdirAll(targetDir, 0777)
	if err != nil {
		panic(err)
	}

	platforms.Each(func(pl Platform) {
		srcBin := "pinpoint-agent"
		if pl.GOOS == "windows" {
			srcBin += ".exe"
		}
		targetBin := "pinpoint-agent-" + pl.Name
		if pl.GOOS == "windows" {
			targetBin += ".exe"
		}
		dir := fjoin(opts.BuildDir, "bin", pl.Name)
		err := fs.CopyFile(fjoin(dir, srcBin), fjoin(targetDir, targetBin))
		if err != nil {
			panic(err)
		}
	})
}

func (s Platforms) Each(cb func(pl Platform)) {
	concurrency := runtime.NumCPU() * 2
	plChan := platformsToChan(s)
	wg := sync.WaitGroup{}
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pl := range plChan {
				cb(pl)
			}
		}()
	}
	wg.Wait()
}

func getCommitSHA() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Stderr = os.Stderr
	b, err := cmd.Output()
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(b))
}

func buildAgent(opts Opts, pl Platform, ldflags string) {
	out := fjoin(opts.BuildDir, "bin", pl.Name, "pinpoint-agent")
	if pl.GOOS == "windows" {
		out += ".exe"
	}
	exe([]string{"GOOS=" + pl.GOOS, "CGO_ENABLED=0"}, "go", "build", "-ldflags", ldflags, "-tags", "prod", "-o", out)
}

func buildIntegration(opts Opts, in string, pl Platform) {
	out := fjoin(opts.BuildDir, "bin", pl.Name, "integrations", in)
	if pl.GOOS == "windows" {
		out += ".exe"
	}

	exe([]string{"GOOS=" + pl.GOOS, "CGO_ENABLED=0"}, "go", "build", "-o", out, "./integrations/"+in)
}

func exe(env []string, command ...string) {
	fmt.Println(env, command)
	cmd := exec.Command(command[0], command[1:]...)

	cmd.Env = append(os.Environ(), env...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}

func platformsToChan(sl []Platform) chan Platform {
	res := make(chan Platform)
	go func() {
		defer close(res)
		for _, a := range sl {
			res <- a
		}
	}()
	return res
}

func stringsToChan(sl []string) chan string {
	res := make(chan string)
	go func() {
		defer close(res)
		for _, a := range sl {
			res <- a
		}
	}()
	return res
}
