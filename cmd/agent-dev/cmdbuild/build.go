package cmdbuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/pinpt/agent.next/pkg/archive"
)

func doBuild(opts Opts, platforms Platforms) {
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

	{
		// create archives
		err := os.MkdirAll(filepath.Join(opts.BuildDir, "archives"), 0777)
		if err != nil {
			panic(err)
		}

		platforms.Each(func(pl Platform) {
			fmt.Println("creating archive for", pl.Name)
			err := archive.ZipDir(filepath.Join(opts.BuildDir, "archives", pl.Name+".zip"), filepath.Join(opts.BuildDir, "bin", pl.Name))
			if err != nil {
				panic(err)
			}
		})

	}
}

type Platforms []Platform

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
	exe([]string{"GOOS=" + pl.GOOS, "CGO_ENABLED=0"}, "go", "build", "-ldflags", ldflags, "-tags", "prod", "-o", filepath.Join(opts.BuildDir, "bin", pl.Name, "pinpoint-agent"))
}

func buildIntegration(opts Opts, in string, pl Platform) {
	o := filepath.Join(opts.BuildDir, "bin", pl.Name, "integrations", in)
	if pl.GOOS == "windows" {
		o += ".exe"
	}
	exe([]string{"GOOS=" + pl.GOOS, "CGO_ENABLED=0"}, "go", "build", "-o", o, "./integrations/"+in)
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
