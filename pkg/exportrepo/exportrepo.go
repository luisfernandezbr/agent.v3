package exportrepo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/pinpt/agent/pkg/filestore"

	"github.com/pinpt/agent/cmd/cmdexport/process"
	"github.com/pinpt/agent/pkg/commitusers"
	"github.com/pinpt/agent/slimrippy/branches"
	"github.com/pinpt/agent/slimrippy/branchmeta"
	"github.com/pinpt/agent/slimrippy/commits"
	"github.com/pinpt/agent/slimrippy/parentsgraph"

	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/expsessions"
	"github.com/pinpt/agent/pkg/fsconf"
	"github.com/pinpt/agent/pkg/gitclone"
	"github.com/pinpt/agent/pkg/ids"
	"github.com/pinpt/agent/pkg/jsonstore"
	"github.com/pinpt/agent/pkg/structmarshal"

	"github.com/hashicorp/go-hclog"
)

type Opts struct {
	Logger     hclog.Logger
	CustomerID string
	RepoID     string

	// UniqueName is a name that will be used in the cache folder name. It should include the all info needed to find the repo in customer org. For example for github it should be NameWithOwner. It's preferrable to have a unique name for integration, but not required since we add id (and also refType) when storing in cache dir.
	UniqueName string

	// RefType to use when creating objects.
	// For example:
	// github, tfs
	RefType string

	LastProcessed *jsonstore.Store
	RepoAccess    gitclone.AccessDetails

	// LocalRepo is a path to local repo for easier testing with agent-dev export-repo
	LocalRepo string

	// CommitURLTemplate is a template for building commit url
	// https://example.com/repo1/@@@sha@@@
	CommitURLTemplate string
	// BranchURLTemplate is a template for building branch url
	// https://example.com/repo1/@@@branch@@@
	BranchURLTemplate string

	Sessions      *expsessions.Manager
	SessionRootID expsessions.ID

	// PRs to process similar to branches.
	PRs []PR

	CommitUsers *process.CommitUsers
}

type PR struct {
	ID            string
	RefID         string
	BranchName    string
	URL           string
	LastCommitSHA string
}

type Export struct {
	opts   Opts
	locs   fsconf.Locs
	logger hclog.Logger

	repoNameUsedInCacheDir string
	lastProcessedKey       []string

	sessions *sessions

	state state
	store filestore.Store
}

func New(opts Opts, locs fsconf.Locs) *Export {
	if opts.Logger == nil || opts.CustomerID == "" || opts.RepoID == "" || opts.RefType == "" || opts.Sessions == nil || opts.LastProcessed == nil || opts.CommitURLTemplate == "" || opts.BranchURLTemplate == "" || opts.CommitUsers == nil {
		panic("provide all params")
	}
	s := &Export{}
	s.opts = opts
	s.logger = opts.Logger.Named("exportrepo")
	s.locs = locs
	return s
}

var ErrRevParseFailed = errors.New("git rev-parse HEAD failed")

type ExportDuration struct {
	Clone  time.Duration
	Ripsrc time.Duration
}

// Result is the result of the export run.
type Result struct {
	// RepoNameUsedInCacheDir name suitable for file system.
	RepoNameUsedInCacheDir string
	// Duration is the information on time taken.
	Duration ExportDuration
	// SessionErr contains an error if it was not possible to open/close sessions.
	// We fail full export on these errors, since these would be related to fs errors
	// and would lead to invalid session files.
	SessionErr error
	// OtherErr is mostly risprc error or other errors in processing that is not related to closing sessions properly.
	OtherErr error
}

func (s *Export) Run(ctx context.Context) (res Result) {
	if s.opts.LocalRepo == "" {
		s.repoNameUsedInCacheDir = gitclone.RepoNameUsedInCacheDir(s.opts.UniqueName, s.opts.RepoID)
	} else {
		s.repoNameUsedInCacheDir = filepath.Base(s.opts.LocalRepo)
	}
	s.logger = s.logger.With("repo", s.repoNameUsedInCacheDir)
	s.lastProcessedKey = []string{"ripsrc-v2", s.repoNameUsedInCacheDir}

	s.sessions = newSessions(s.opts.Sessions, s.opts.SessionRootID, s.repoNameUsedInCacheDir)

	err := s.sessions.Open()
	if err != nil {
		res.SessionErr = err
		return
	}

	res.Duration, res.OtherErr = s.run(ctx)

	err = s.sessions.Close()
	if err != nil {
		res.SessionErr = err
		return
	}

	return
}

type state struct {
	Commits commits.State
}

func (s *Export) loadState() error {
	s.store = filestore.New(s.locs.RipsrcCheckpoints)
	err := s.store.Get(s.opts.RepoID, &s.state)
	if err != nil {
		panic(err)
	}
	fmt.Println("state commits", len(s.state.Commits.CommitsSeen))
	return nil
}

func (s *Export) saveState() error {
	return s.store.Set(s.opts.RepoID, s.state)
}

func (s *Export) lastProcessedGet(keyLocal ...string) interface{} {
	key := append(s.lastProcessedKey, keyLocal...)
	return s.opts.LastProcessed.Get(key...)
}

func (s *Export) lastProcessedSet(val interface{}, keyLocal ...string) error {
	key := append(s.lastProcessedKey, keyLocal...)
	return s.opts.LastProcessed.Set(val, key...)
}

const lpBranches = "branches"

func (s *Export) run(ctx context.Context) (duration ExportDuration, rerr error) {
	err := os.MkdirAll(s.locs.Temp, 0777)
	if err != nil {
		rerr = err
		return
	}
	s.logger.Debug("git clone started", "repo", s.opts.UniqueName)
	clonestarted := time.Now()
	repoDir, err := s.clone(ctx)
	if err != nil {
		rerr = err
		return
	}

	duration.Clone = time.Since(clonestarted)
	s.logger.Debug("git clone finished", "duration", duration.Clone.String(), "repo", s.opts.UniqueName)
	if !hasHeadCommit(ctx, repoDir) {
		rerr = ErrRevParseFailed
		return
	}

	skipsrc, remotebranches, err := s.skipRipsrc(ctx, repoDir)
	if err != nil {
		rerr = err
		return
	}
	if skipsrc {
		s.logger.Info("no changes to this repo, skipping ripsrc")
		return
	}

	err = s.loadState()
	if err != nil {
		rerr = err
		return
	}

	branchesStarted := time.Now()
	s.logger.Info("processing branches")
	if err = s.branches(ctx, repoDir); err != nil {
		rerr = fmt.Errorf("branch processing failed: %v", err)
		return
	}
	s.logger.Info("processing branches done", "d", time.Since(branchesStarted))

	codeStarted := time.Now()
	s.logger.Info("processing code")
	err = s.code(ctx, repoDir)
	if err != nil {
		rerr = fmt.Errorf("code processing failed: %v", err)
		return
	}
	s.logger.Info("processing code done", "d", time.Since(codeStarted))

	duration.Ripsrc = time.Since(branchesStarted)
	s.logger.Info("ripsrc finished", "duration", duration.Ripsrc, "repo", s.opts.UniqueName)

	err = s.saveState()
	if err != nil {
		rerr = err
		return
	}
	rerr = s.lastProcessedSet(remotebranches, lpBranches)
	return
}

func hasHeadCommit(ctx context.Context, repoDir string) bool {
	out := bytes.NewBuffer(nil)
	c := exec.Command("git", "rev-parse", "HEAD")
	c.Dir = repoDir
	c.Stdout = out
	c.Run()
	res := strings.TrimSpace(out.String())
	if len(res) != 40 {
		return false
	}
	return true
}

func (s *Export) clone(ctx context.Context) (
	tempCheckoutDir string,
	_ error) {

	if s.opts.LocalRepo != "" {
		return s.opts.LocalRepo, nil
	}

	uniqueName := s.opts.RefType + "-" + s.opts.UniqueName

	/*
		tempDir, err := ioutil.TempDir(s.locs.Temp, "exportrepo")
		if err != nil {
			return "", err
		}*/

	dirs := gitclone.Dirs{
		CacheRoot: s.locs.RepoCache,
		//Checkout:  tempDir,
	}
	res, err := gitclone.CloneWithCache(ctx, s.logger, s.opts.RepoAccess, dirs, s.opts.RepoID, uniqueName)

	if err != nil {
		return "", err
	}

	return res.Checkout, nil
}

func (s *Export) skipRipsrc(ctx context.Context, checkoutdir string) (bool, map[string]branchmeta.Branch, error) {
	cachedbranches := make(map[string]branchmeta.Branch)
	remotebranches := make(map[string]branchmeta.Branch)
	cached := s.lastProcessedGet(lpBranches)
	if cached != nil {
		if err := structmarshal.StructToStruct(cached, &cachedbranches); err != nil {
			return true, nil, err
		}
	}
	opts := branchmeta.Opts{
		RepoDir:        checkoutdir,
		UseOrigin:      true,
		IncludeDefault: true,
	}
	br, err := branchmeta.Get(ctx, opts)
	if err != nil {
		return true, nil, fmt.Errorf("branchmeta.Get %v", err)
	}
	for _, b := range br {
		remotebranches[b.Name] = branchmeta.Branch{
			Name:   b.Name,
			Commit: b.Commit,
		}
	}
	if len(remotebranches) == 0 {
		return false, remotebranches, nil
	}
	skip := reflect.DeepEqual(cachedbranches, remotebranches)
	return skip, remotebranches, nil
}

func (s *Export) branches(ctx context.Context, checkoutdir string) error {
	sessions := s.opts.Sessions

	res := make(chan branches.Branch)
	done := make(chan error)

	prs := map[string]PR{}
	for _, pr := range s.opts.PRs {
		prs[pr.LastCommitSHA] = pr
	}

	go func() {

		for data := range res {
			if len(data.Commits) == 0 {
				// we do not export branches with no commits, especially since branch id depends on first commit
				continue
			}

			commitIDs := s.commitIDs(data.Commits)
			var pr PR
			isPr := data.IsPullRequest

			if isPr {
				var ok bool
				pr, ok = prs[data.HeadSHA]
				if !ok {
					s.logger.Error("could not find pr by sha")
					continue
				}
				obj := sourcecode.PullRequestBranch{
					PullRequestID:          pr.ID,
					RefID:                  pr.RefID,
					Name:                   pr.BranchName,
					URL:                    pr.URL,
					RefType:                s.opts.RefType,
					CustomerID:             s.opts.CustomerID,
					Default:                data.IsDefault,
					Merged:                 data.IsMerged,
					MergeCommitSha:         data.MergeCommit,
					MergeCommitID:          s.commitID(data.MergeCommit),
					BranchedFromCommitShas: data.BranchedFromCommits,
					BranchedFromCommitIds:  s.commitIDs(data.BranchedFromCommits),
					CommitShas:             data.Commits,
					CommitIds:              commitIDs,
					BehindDefaultCount:     int64(data.BehindDefaultCount),
					AheadDefaultCount:      int64(data.AheadDefaultCount),
					RepoID:                 s.opts.RepoID,
				}
				err := sessions.Write(s.sessions.PRBranch, []map[string]interface{}{
					obj.ToMap(),
				})
				if err != nil {
					done <- err
					return
				}
			} else {
				obj := sourcecode.Branch{
					RefID:                  data.Name,
					Name:                   data.Name,
					URL:                    branchURL(s.opts.BranchURLTemplate, data.Name),
					RefType:                s.opts.RefType,
					CustomerID:             s.opts.CustomerID,
					Default:                data.IsDefault,
					Merged:                 data.IsMerged,
					MergeCommitSha:         data.MergeCommit,
					MergeCommitID:          s.commitID(data.MergeCommit),
					BranchedFromCommitShas: data.BranchedFromCommits,
					BranchedFromCommitIds:  s.commitIDs(data.BranchedFromCommits),
					CommitShas:             data.Commits,
					CommitIds:              commitIDs,
					FirstCommitSha:         data.FirstCommit,
					FirstCommitID:          s.commitID(data.FirstCommit),
					BehindDefaultCount:     int64(data.BehindDefaultCount),
					AheadDefaultCount:      int64(data.AheadDefaultCount),
					RepoID:                 s.opts.RepoID,
				}
				err := sessions.Write(s.sessions.Branch, []map[string]interface{}{
					obj.ToMap(),
				})
				if err != nil {
					done <- err
					return
				}
			}
		}
		done <- nil
	}()

	commitGraph := parentsgraph.New(parentsgraph.Opts{
		RepoDir:     checkoutdir,
		AllBranches: true,
		UseOrigin:   true,
	})

	err := commitGraph.Read()
	if err != nil {
		return err
	}

	opts := branches.Opts{}
	opts.UseOrigin = true
	opts.CommitGraph = commitGraph
	opts.RepoDir = checkoutdir
	opts.IncludeDefaultBranch = true
	var prSHAs []string
	for _, pr := range s.opts.PRs {
		prSHAs = append(prSHAs, pr.LastCommitSHA)
	}
	opts.PullRequestSHAs = prSHAs
	pr := branches.New(opts)
	err = pr.Run(ctx, res)
	err2 := <-done

	if err != nil {
		return err
	}

	if err2 != nil {
		return err2
	}

	return nil
}

func (s *Export) commitID(sha string) string {
	if sha == "" {
		return ""
	}
	return ids.CodeCommit(s.opts.CustomerID, s.opts.RefType, s.opts.RepoID, sha)
}

func (s *Export) commitIDs(shas []string) (res []string) {
	return ids.CodeCommits(s.opts.CustomerID, s.opts.RefType, s.opts.RepoID, shas)
}

const lpCommit = "commit"

func (s *Export) code(ctx context.Context, checkoutDir string) error {
	started := time.Now()

	res := make(chan commits.Commit)
	done := make(chan bool)
	go func() {
		defer func() { done <- true }()
		err := s.processCode(res)
		if err != nil {
			panic(err)
		}
	}()

	opts := commits.Opts{}
	opts.State = s.state.Commits
	opts.RepoDir = checkoutDir
	opts.UseOrigin = true
	state, err := commits.Commits(ctx, opts, res)
	if err != nil {
		return err
	}

	<-done

	s.state.Commits = state

	s.logger.Debug("code processing end", "duration", time.Since(started))

	return nil

}

func (s *Export) processCode(commits chan commits.Commit) error {
	sessions := s.opts.Sessions

	writeCommit := func(obj sourcecode.Commit) error {
		return sessions.Write(s.sessions.Commit, []map[string]interface{}{
			obj.ToMap(),
		})
	}

	writeCommitUser := func(obj commitusers.CommitUser) error {
		obj2, err := s.opts.CommitUsers.Transform(obj.ToMap())
		if err != nil {
			return err
		}
		// already written before
		if obj2 == nil {
			return nil
		}
		return sessions.Write(s.sessions.CommitUser, []map[string]interface{}{
			obj2,
		})
	}

	customerID := s.opts.CustomerID

	repoID := s.opts.RepoID

	for commit := range commits {
		c := sourcecode.Commit{
			RefID:          commit.SHA,
			RefType:        s.opts.RefType,
			CustomerID:     customerID,
			RepoID:         repoID,
			Sha:            commit.SHA,
			Message:        commit.Message,
			URL:            commitURL(s.opts.CommitURLTemplate, commit.SHA),
			AuthorRefID:    ids.CodeCommitEmail(customerID, commit.Authored.Email),
			CommitterRefID: ids.CodeCommitEmail(customerID, commit.Committed.Email),
		}

		date.ConvertToModel(commit.Committed.Date, &c.CreatedDate)

		err := writeCommit(c)
		if err != nil {
			return err
		}

		if commit.Authored.Email != "" {
			author := commitusers.CommitUser{}
			author.CustomerID = customerID
			author.Email = commit.Authored.Email
			author.Name = commit.Authored.Name
			err := writeCommitUser(author)
			if err != nil {
				return err
			}
		}

		if commit.Committed.Email != "" {
			author := commitusers.CommitUser{}
			author.CustomerID = customerID
			author.Email = commit.Committed.Email
			author.Name = commit.Committed.Name
			err := writeCommitUser(author)
			if err != nil {
				return err
			}
		}

	}

	return nil
}

func commitURL(commitURLTemplate, sha string) string {
	return strings.ReplaceAll(commitURLTemplate, "@@@sha@@@", sha)
}

func branchURL(branchURLTemplate, branchName string) string {
	return strings.ReplaceAll(branchURLTemplate, "@@@branch@@@", branchName)
}
