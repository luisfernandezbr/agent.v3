package gitclone

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/v10/fileutil"
)

type AccessDetails struct {
	URL string
}

type Dirs struct {
	CacheRoot string
}

type CloneResults struct {
	CacheDir string
	Checkout string
}

func RepoNameUsedInCacheDir(repoName, repoID string) string {
	return escapeForFS(repoName) + "-" + repoID
}

// CloneWithCache mirrors a provided repo into dirs.CacheRoot/name-repoID. And then checkout a copy into dirs.Checkout
// Automatically retries on errors.
func CloneWithCache(ctx context.Context, logger hclog.Logger, access AccessDetails, dirs Dirs, repoID string, name string) (_ CloneResults, rerr error) {
	logger = logger.Named("git")

	dirName := RepoNameUsedInCacheDir(name, repoID)
	logger = logger.With("repo", dirName)

	started := time.Now()
	logger.Debug("CloneWithCache")
	defer func() {
		logger = logger.With("duration", time.Since(started))
		if rerr != nil {
			logger.Debug("CloneWithCache failed", "err", rerr)
			return
		}
		logger.Debug("CloneWithCache success")
	}()

	maxAttempts := 3
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if i != 0 {
			time.Sleep(time.Duration(i*i) * time.Minute)
		}
		res, err := cloneWithCacheNoRetries(ctx, logger, access, dirs, dirName)
		if err == nil {
			return res, nil
		}
		if strings.Contains(err.Error(), "Access denied") {
			rerr = fmt.Errorf("CloneWithCache failed with Access denied, not retrying: %v", err)
			return
		}
		lastErr = err
		logger.Warn("CloneWithCache failed attempt", "n", i, "err", err)
	}

	rerr = fmt.Errorf("failed multiple attempts at git clone, last error: %v", lastErr)
	return
}

func cloneWithCacheNoRetries(ctx context.Context, logger hclog.Logger, access AccessDetails, dirs Dirs, cacheDirName string) (res CloneResults, rerr error) {
	if dirs.CacheRoot == "" {
		panic("provide CacheRoot")
	}
	if cacheDirName == "" {
		panic("provide cacheDirName")
	}
	started := time.Now()
	logger.Debug("cloneWithCacheNoRetries")
	defer func() {
		logger = logger.With("duration", time.Since(started).String())
		if rerr != nil {
			logger.Debug("cloneWithCacheNoRetries failed", "err", rerr)
			return
		}
		logger.Debug("cloneWithCacheNoRetries success")
	}()

	cacheDir := filepath.Join(dirs.CacheRoot, cacheDirName)

	if !fileutil.FileExists(cacheDir) {
		logger.Info("git clone if exist")
		err := cloneFreshIntoCache(ctx, logger, access, dirs, cacheDirName)
		if err != nil {
			rerr = err
			return
		}
	} else {
		logger.Info("git clone updating credentials")
		err := updateCredentials(ctx, logger, access, dirs, cacheDirName)
		if err != nil {
			rerr = err
			return
		}
		logger.Info("git clone updating credentials")
		err = updateClonedRepo(ctx, logger, access, dirs, cacheDirName)
		if err != nil {
			rerr = err
			return
		}
	}
	res.Checkout = cacheDir
	return
}

func updateCredentials(ctx context.Context, logger hclog.Logger, access AccessDetails, dirs Dirs, cacheDirName string) error {
	logger.Debug("updateCredentials")
	cacheDir := filepath.Join(dirs.CacheRoot, cacheDirName)
	cmd := exec.CommandContext(ctx, "git", "remote", "set-url", "origin", access.URL)
	cmd.Dir = cacheDir
	err := runGitCommand(ctx, logger, cmd)
	if err != nil {
		return err
	}
	return nil
}

func cloneFreshIntoCache(ctx context.Context, logger hclog.Logger, access AccessDetails, dirs Dirs, cacheDirName string) error {
	logger.Debug("cloneFreshIntoCache")
	cloneStarted := time.Now()
	tempDir := filepath.Join(dirs.CacheRoot, "tmp", cacheDirName)
	err := os.RemoveAll(tempDir)
	if err != nil {
		return err
	}
	args := []string{"clone", "-c", "core.longpaths=true", "--mirror", access.URL, tempDir}

	args = append(args, cloneArgs(access.URL)...)
	cmd := exec.CommandContext(ctx, "git", args...)
	err = runGitCommand(ctx, logger, cmd)
	if err != nil {
		output, err := RedactCredsInText(err.Error(), access.URL)
		if err != nil {
			return err
		}
		return errors.New(output)
	}
	// set the git config, so the further updates would use the same config as initial clone
	err = setRepoConfig(ctx, logger, access.URL, tempDir)
	if err != nil {
		return err
	}
	if time.Since(cloneStarted) > time.Duration(30)*time.Second {
		logger.Debug("running git gc because clone took >30s", "duration", time.Since(cloneStarted))
		err := gitRunGCForLongClone(ctx, logger, tempDir)
		if err != nil {
			return err
		}
	}
	// move into final location
	return os.Rename(tempDir, filepath.Join(dirs.CacheRoot, cacheDirName))
}

func updateClonedRepo(ctx context.Context, logger hclog.Logger, access AccessDetails, dirs Dirs, cacheDirName string) error {
	logger.Debug("updateClonedRepo")
	cacheDir := filepath.Join(dirs.CacheRoot, cacheDirName)
	cmd := exec.CommandContext(ctx, "git", "remote", "update", "--prune")
	cmd.Dir = cacheDir
	err := runGitCommand(ctx, logger, cmd)
	if err != nil {
		return err
	}

	// we run into a case where the mirror can become out dated from origin
	// (for example default branch is deleted) and we need to re-mirror
	// from origin. the easiest test for this is to check for git log
	// in the mirror and if it fails, we just blow away and start clean
	cmd = exec.CommandContext(ctx, "git", "log", "-n", "1")
	cmd.Dir = cacheDir
	err = runGitCommand(ctx, logger, cmd)
	if err != nil {
		logger.Info("detected a git mirror which needs to be updated, will do a fresh reclone")
		os.RemoveAll(cacheDir)
		return cloneFreshIntoCache(ctx, logger, access, dirs, cacheDirName)
	}

	return nil
}

func gitRunGCForLongClone(ctx context.Context, logger hclog.Logger, dir string) error {
	cmd := exec.CommandContext(ctx, "git", "gc")
	cmd.Dir = dir
	gcStarted := time.Now()
	err := runGitCommand(ctx, logger, cmd)
	if err != nil {
		return err
	}
	logger.Debug("git gc on repo", "duration", time.Since(gcStarted))
	return nil
}

func runGitCommand(ctx context.Context, logger hclog.Logger, cmd *exec.Cmd) error {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		c := ""
		if len(cmd.Args) > 2 {
			c = cmd.Args[1]
		}
		//logger.Info("git command failed", "command", c, "output", stderr.String())
		return fmt.Errorf("git command failed, command %v, output %v", c, stderr.String())
	}
	return nil
}

func makeConfig(repoURL string) map[string]string {
	res := map[string]string{}
	// abort connection if speed is lower than 10KB/s for 1m (we would retry)
	res["http.lowSpeedLimit"] = "10000"
	res["http.lowSpeedTime"] = "60"

	if !(strings.Contains(repoURL, "api.github.com") || strings.Contains(repoURL, "gitlab.com") || strings.Contains(repoURL, "bitbucket.org")) {
		// for enterprise we need to support self-signed certs
		res["http.sslVerify"] = "false"
	}

	return res
}

func cloneArgs(repoURL string) (args []string) {
	conf := makeConfig(repoURL)
	for k, v := range conf {
		args = append(args, "-c", k+"="+v)
	}
	return
}

func setRepoConfig(ctx context.Context, logger hclog.Logger, repoURL, repoDir string) error {

	conf := makeConfig(repoURL)

	for k, v := range conf {
		cmd := exec.CommandContext(ctx, "git", "config", k, v)
		cmd.Dir = repoDir
		err := runGitCommand(ctx, logger, cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

var alphaNumericRe = regexp.MustCompile(`[^a-zA-Z\d]`)

func escapeForFS(name string) string {
	return alphaNumericRe.ReplaceAllString(name, "-")
}

func RedactCredsInText(text string, urlStr string) (redactedText string, _ error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	if u.User.String() == "" {
		return text, nil
	}
	res := strings.Replace(text, u.User.String(), "[redacted]", -1)
	return res, nil
}
