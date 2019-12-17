package exportrepo

import (
	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/pinpt/agent/pkg/expsessions"
)

type sessions struct {
	Branch   expsessions.ID
	PRBranch expsessions.ID
	Blame    expsessions.ID
	Commit   expsessions.ID

	sessionManager         *expsessions.Manager
	sessionRootID          expsessions.ID
	repoNameUsedInCacheDir string
}

func newSessions(sessionManager *expsessions.Manager, sessionRootID expsessions.ID, repoNameUsedInCacheDir string) *sessions {
	s := &sessions{}
	s.sessionManager = sessionManager
	s.sessionRootID = sessionRootID
	s.repoNameUsedInCacheDir = repoNameUsedInCacheDir
	return s
}

func (s *sessions) Open() error {
	var err error
	s.Branch, err = s.session(sourcecode.BranchModelName.String())
	if err != nil {
		return err
	}
	s.PRBranch, err = s.session(sourcecode.PullRequestBranchModelName.String())
	if err != nil {
		return err
	}
	s.Blame, err = s.session(sourcecode.BlameModelName.String())
	if err != nil {
		return err
	}
	s.Commit, err = s.session(sourcecode.CommitModelName.String())
	if err != nil {
		return err
	}
	return nil
}

func (s *sessions) Close() error {
	err := s.sessionManager.Done(s.Branch, nil)
	if err != nil {
		return err
	}
	err = s.sessionManager.Done(s.PRBranch, nil)
	if err != nil {
		return err
	}
	err = s.sessionManager.Done(s.Blame, nil)
	if err != nil {
		return err
	}
	err = s.sessionManager.Done(s.Commit, nil)
	if err != nil {
		return err
	}
	return nil
}

func (s *sessions) session(model string) (expsessions.ID, error) {
	id, _, err := s.sessionManager.Session(
		model, s.sessionRootID,
		s.repoNameUsedInCacheDir,
		s.repoNameUsedInCacheDir)

	return id, err
}
