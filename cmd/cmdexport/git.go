package cmdexport

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pinpt/agent.next/pkg/exportrepo"
	"github.com/pinpt/agent.next/pkg/gitclone"
	"github.com/pinpt/agent.next/pkg/integrationid"
)

func (s *export) gitProcessing() (hadErrors bool, fatalError error) {
	logger := s.Logger.Named("git")

	if s.Opts.AgentConfig.SkipGit {
		logger.Warn("SkipGit is true, skipping git clone and ripsrc for all repos")
		for range s.gitProcessingRepos {
		}
		return
	}

	// force kill git processing if this process is killed
	sigkill := make(chan os.Signal, 1)
	signal.Notify(sigkill, syscall.SIGKILL)
	go func() {
		sig := <-sigkill
		s.Logger.Info("exporter killed manually", "sig", sig.String())
		gitclone.RemoveAllProcesses()
	}()

	logger.Info("starting git/ripsrc repo processing")

	i := 0
	reposFailedRevParse := 0
	var start time.Time

	ctx := context.Background()
	sessionRoot, _, err := s.sessions.expsession.SessionRootTracking(integrationid.ID{
		Name: "git",
	}, "git")
	if err != nil {
		logger.Error("could not create session for git export", "err", err.Error())
		fatalError = err
		return
	}

	resErrors := map[string]error{}
	var ripsrcDuration time.Duration
	var gitClonecDuration time.Duration
	for fetch := range s.gitProcessingRepos {
		if i == 0 {
			start = time.Now()
		}
		i++
		access := gitclone.AccessDetails{}
		access.URL = fetch.URL

		opts := exportrepo.Opts{
			Logger:     s.Logger.With("c", i),
			CustomerID: s.Opts.AgentConfig.CustomerID,
			RepoID:     fetch.RepoID,
			UniqueName: fetch.UniqueName,
			RefType:    fetch.RefType,

			LastProcessed: s.lastProcessed,
			RepoAccess:    access,

			CommitURLTemplate: fetch.CommitURLTemplate,
			BranchURLTemplate: fetch.BranchURLTemplate,

			Sessions:      s.sessions.expsession,
			SessionRootID: sessionRoot,
		}
		for _, pr1 := range fetch.PRs {
			pr2 := exportrepo.PR{}
			pr2.ID = pr1.ID
			pr2.RefID = pr1.RefID
			pr2.URL = pr1.URL
			pr2.BranchName = pr1.BranchName
			pr2.LastCommitSHA = pr1.LastCommitSHA
			opts.PRs = append(opts.PRs, pr2)
		}
		exp := exportrepo.New(opts, s.Locs)
		runResult := exp.Run(ctx)
		if runResult.SessionErr != nil {
			fatalError = err
			return
		}
		repoDirName := runResult.RepoNameUsedInCacheDir
		err := runResult.OtherErr
		s.sessions.expsession.Progress(sessionRoot, i, 0)
		if err == exportrepo.ErrRevParseFailed {
			reposFailedRevParse++
			continue
		}
		if err != nil {
			logger.Error("Error processing git repo", "repo", repoDirName, "err", err)
			resErrors[repoDirName] = err
		} else {
			logger.Info("Finished processing git repo", "repo", repoDirName)
		}
		duration := runResult.Duration
		ripsrcDuration += duration.Ripsrc
		gitClonecDuration += duration.Clone
	}

	if i == 0 {
		logger.Info("Finished git repo processing: No git repos found")
		return
	}

	if reposFailedRevParse != 0 {
		logger.Warn("Skipped ripsrc on empty repos", "repos", reposFailedRevParse)
	}

	if len(resErrors) != 0 {
		for k, err := range resErrors {
			logger.Error("Error processing git repo", "repo", k, "err", err)
		}
		logger.Error("Error in git repo processing", "count", i, "dur", time.Since(start).String(), "repos_failed", len(resErrors))

		if len(resErrors) > 5 {
			fatalError = fmt.Errorf("More than 5 repos errored on git/ripsrc, failing export. Failed: %v", len(resErrors))
			return
		}

		hadErrors = true
		return
	}

	err = s.sessions.expsession.Done(sessionRoot, nil)
	if err != nil {
		logger.Error("could not close session for git export", "err", err.Error())
		fatalError = err
		return
	}

	logger.Info("Finished git repo processing", "count", i,
		"duration", time.Since(start).String(),
		"gitclone", gitClonecDuration.String(),
		"ripsrc", ripsrcDuration.String(),
	)

	return false, nil
}
