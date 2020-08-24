package cmdexport

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/expin"
	"github.com/pinpt/agent/pkg/expsessions"
	"github.com/pinpt/agent/pkg/gitclone"
	"github.com/pinpt/agent/slimrippy/exportrepo"
)

func (s *export) gitSession(logger hclog.Logger, exp expin.Export) (_ expsessions.ID, rerr error) {
	if s.gitSessions == nil {
		s.gitSessions = map[expin.Export]expsessions.ID{}
	}

	if sessID, ok := s.gitSessions[exp]; ok {
		return sessID, nil
	}

	sessID, _, err := s.sessions.expsession.SessionRootTracking(exp, "git")
	if err != nil {
		logger.Error("could not create session for git export", "integration", exp.String(), "err", err.Error())
		rerr = err
		return
	}

	s.gitSessions[exp] = sessID
	return sessID, nil
}

func (s *export) gitSessionsClose(logger hclog.Logger) error {
	for exp, sessID := range s.gitSessions {
		err := s.sessions.expsession.Done(sessID, nil)
		if err != nil {
			logger.Error("could not close session for git export", "integration", exp.String(), "err", err.Error())
			return err
		}
	}
	return nil
}

func (s *export) gitSetResult(exp expin.Export, repoID string, err error) {
	if s.gitResults == nil {
		s.gitResults = map[expin.Export]map[string]error{}
	}
	if _, ok := s.gitResults[exp]; !ok {
		s.gitResults[exp] = map[string]error{}
	}
	s.gitResults[exp][repoID] = err
}

func (s *export) gitProcessing() (hadErrors bool, fatalError error) {
	logger := s.Logger.Named("git")

	// if s.Opts.AgentConfig.SkipGit {
	logger.Warn("SkipGit is true, skipping git clone and ripsrc for all repos")
	for range s.gitProcessingRepos {
	}
	return
	// }

	// if s.deviceInfo.CustomerID == "a81aa38c034c4d89" ||
	// 	s.deviceInfo.CustomerID == "0cd907eefb614a29" ||
	// 	s.deviceInfo.CustomerID == "b3d0b0050c3b7f6c" {
	// 	logger.Warn("SkipGit is true, skipping git clone and ripsrc for all repos")
	// 	for range s.gitProcessingRepos {
	// 	}
	// 	return
	// }

	logger.Info("starting git/ripsrc repo processing")

	i := 0
	reposFailedRevParse := 0
	var start time.Time

	ctx := context.Background()

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

		sessionID, err := s.gitSession(logger, fetch.exp)
		if err != nil {
			fatalError = err
			return
		}

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
			SessionRootID: sessionID,

			CommitUsers: s.sessions.commitUsers,
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
		err = runResult.OtherErr
		s.gitSetResult(fetch.exp, fetch.RepoID, err)
		s.sessions.expsession.Progress(sessionID, i, 0)
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

	err := s.gitSessionsClose(logger)
	if err != nil {
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
