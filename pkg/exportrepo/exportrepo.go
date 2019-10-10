package exportrepo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pinpt/go-common/datetime"
	"github.com/pinpt/go-common/fileutil"

	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/expsessions"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/agent.next/pkg/gitclone"
	"github.com/pinpt/agent.next/pkg/ids"
	"github.com/pinpt/agent.next/pkg/jsonstore"
	"github.com/pinpt/ripsrc/ripsrc"

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

	Sessions *expsessions.Manager

	LastProcessed *jsonstore.Store
	RepoAccess    gitclone.AccessDetails

	// CommitURLTemplate is a template for building commit url
	// https://example.com/repo1/@@@sha@@@
	CommitURLTemplate string
	// BranchURLTemplate is a template for building branch url
	// https://example.com/repo1/@@@branch@@@
	BranchURLTemplate string
}

type Export struct {
	opts   Opts
	locs   fsconf.Locs
	logger hclog.Logger
	//defaultBranch string

	repoNameUsedInCacheDir string
	lastProcessedKey       []string

	rip *ripsrc.Ripsrc
}

func New(opts Opts, locs fsconf.Locs) *Export {
	if opts.Logger == nil || opts.CustomerID == "" || opts.RepoID == "" || opts.RefType == "" || opts.Sessions == nil || opts.LastProcessed == nil || opts.RepoAccess.URL == "" || opts.CommitURLTemplate == "" || opts.BranchURLTemplate == "" {
		panic("provide all params")
	}
	s := &Export{}
	s.opts = opts
	s.logger = opts.Logger.Named("exportrepo")
	s.locs = locs
	return s
}

var ErrRevParseFailed = errors.New("git rev-parse HEAD failed")

func (s *Export) Run(ctx context.Context) (repoNameUsedInCacheDir string, rerr error) {
	err := os.MkdirAll(s.locs.Temp, 0777)
	if err != nil {
		rerr = err
		return
	}

	checkoutDir, cacheDir, err := s.clone(ctx)
	if err != nil {
		rerr = err
		return
	}

	if !hasHeadCommit(ctx, checkoutDir) {
		rerr = ErrRevParseFailed
		return
	}

	s.repoNameUsedInCacheDir = filepath.Base(cacheDir)
	repoNameUsedInCacheDir = s.repoNameUsedInCacheDir

	s.logger = s.logger.With("repo", s.repoNameUsedInCacheDir)

	s.ripsrcSetup(checkoutDir)

	err = s.branches(ctx)
	if err != nil {
		rerr = err
		return
	}

	err = s.code(ctx)
	if err != nil {
		rerr = err
		return
	}

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
	checkoutDir string,
	cacheDir string,
	_ error) {

	uniqueName := s.opts.RefType + "-" + s.opts.UniqueName

	tempDir, err := ioutil.TempDir(s.locs.Temp, "")
	if err != nil {
		return "", "", err
	}

	dirs := gitclone.Dirs{
		CacheRoot: s.locs.RepoCache,
		Checkout:  tempDir,
	}
	res, err := gitclone.CloneWithCache(ctx, s.logger, s.opts.RepoAccess, dirs, s.opts.RepoID, uniqueName)

	if err != nil {
		return "", "", err
	}

	return tempDir, res.CacheDir, nil
}

func (s *Export) ripsrcSetup(repoDir string) {

	opts := ripsrc.Opts{}
	opts.Logger = s.logger.Named("ripsrc")
	opts.RepoDir = repoDir
	opts.AllBranches = true
	opts.BranchesUseOrigin = true
	opts.CheckpointsDir = filepath.Join(s.locs.RipsrcCheckpoints, s.repoNameUsedInCacheDir)

	s.lastProcessedKey = []string{"ripsrc", s.repoNameUsedInCacheDir}

	lastCommit := s.opts.LastProcessed.Get(s.lastProcessedKey...)
	if lastCommit != nil {
		opts.CommitFromIncl = lastCommit.(string)
		opts.CommitFromMakeNonIncl = true

		if !fileutil.FileExists(opts.CheckpointsDir) {
			panic(fmt.Errorf("expecting to run incrementals, but ripsrc checkpoints dir does not exist for repo: %v", s.repoNameUsedInCacheDir))
		}
	}

	s.logger.Info("setting up ripsrc", "last_processed_old", lastCommit)

	s.rip = ripsrc.New(opts)
}

func (s *Export) branches(ctx context.Context) error {
	sessions := s.opts.Sessions
	sessionID, _, err := sessions.SessionRoot(sourcecode.BranchModelName.String())
	if err != nil {
		return err
	}
	defer sessions.Done(sessionID, nil)

	res := make(chan ripsrc.Branch)
	done := make(chan bool)

	go func() {
		for data := range res {
			if len(data.Commits) == 0 {
				// we do not export branches with no commits, especially since branch id depends on first commit
				continue
			}

			obj := sourcecode.Branch{}
			obj.RefID = data.Name
			obj.RefType = s.opts.RefType
			obj.CustomerID = s.opts.CustomerID
			obj.Name = data.Name
			obj.URL = branchURL(s.opts.BranchURLTemplate, data.Name)
			obj.Default = data.IsDefault
			obj.Merged = data.IsMerged
			obj.MergeCommitSha = data.MergeCommit
			obj.MergeCommitID = s.commitID(obj.MergeCommitSha)
			obj.BranchedFromCommitShas = data.BranchedFromCommits
			obj.BranchedFromCommitIds = s.commitIDs(obj.BranchedFromCommitShas)
			if len(obj.BranchedFromCommitShas) != 0 {
				// this aren't really used in the pipeline
				// TODO: remove from datamodel and pipeline
				obj.FirstBranchedFromCommitSha = obj.BranchedFromCommitShas[0]
				obj.FirstBranchedFromCommitID = obj.BranchedFromCommitIds[0]
			}
			obj.CommitShas = data.Commits
			obj.CommitIds = s.commitIDs(obj.CommitShas)
			obj.FirstCommitSha = obj.CommitShas[0]
			obj.FirstCommitID = obj.CommitIds[0]
			obj.BehindDefaultCount = int64(data.BehindDefaultCount)
			obj.AheadDefaultCount = int64(data.AheadDefaultCount)
			obj.RepoID = s.opts.RepoID

			err := sessions.Write(sessionID, []map[string]interface{}{
				obj.ToMap(),
			})
			if err != nil {
				panic(err)
			}
		}
		done <- true
	}()

	err = s.rip.Branches(ctx, res)
	<-done

	if err != nil {
		return err
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

func (s *Export) code(ctx context.Context) error {
	started := time.Now()

	res := make(chan ripsrc.CommitCode, 100)
	done := make(chan bool)

	lastProcessed := ""
	go func() {
		defer func() { done <- true }()
		var err error
		lastProcessed, err = s.processCode(res)
		if err != nil {
			panic(err)
		}
	}()

	err := s.rip.CodeByCommit(ctx, res)
	if err != nil {
		return err
	}
	<-done

	if lastProcessed != "" {
		err := s.opts.LastProcessed.Set(lastProcessed, s.lastProcessedKey...)
		if err != nil {
			return err
		}
	}

	s.logger.Debug("code processing end", "duration", time.Since(started), "last_processed_new", lastProcessed)

	return nil

}

func (s *Export) processCode(commits chan ripsrc.CommitCode) (lastProcessedSHA string, _ error) {
	sessions := s.opts.Sessions
	blameSession, _, err := sessions.SessionRoot(sourcecode.BlameModelName.String())
	if err != nil {
		return "", err
	}
	commitSession, _, err := sessions.SessionRoot(sourcecode.CommitModelName.String())
	if err != nil {
		return "", err
	}

	defer func() {
		err := sessions.Done(blameSession, nil)
		if err != nil {
			panic(err)
		}
		err = sessions.Done(commitSession, nil)
		if err != nil {
			panic(err)
		}
	}()

	writeBlame := func(obj sourcecode.Blame) error {
		return sessions.Write(blameSession, []map[string]interface{}{
			obj.ToMap(),
		})
	}
	writeCommit := func(obj sourcecode.Commit) error {
		return sessions.Write(commitSession, []map[string]interface{}{
			obj.ToMap(),
		})
	}

	var commitAdditions int64
	var commitDeletions int64
	var commitCommentsCount int64
	var commitFilesCount int64
	var commitSlocCount int64
	var commitLocCount int64
	var commitBlanksCount int64
	var commitSize int64
	var commitComplexityCount int64

	customerID := s.opts.CustomerID

	repoID := s.opts.RepoID

	for commit := range commits {
		lastProcessedSHA = commit.SHA

		commitAdditions = 0
		commitDeletions = 0
		commitCommentsCount = 0
		commitFilesCount = 0
		commitSlocCount = 0
		commitLocCount = 0
		commitBlanksCount = 0
		commitComplexityCount = 0

		ordinal := datetime.EpochNow()
		commitFiles := []sourcecode.CommitFiles{}
		var excludedFilesCount int64
		for blame := range commit.Blames {
			//var license string
			var licenseConfidence float32
			if blame.License != nil {
				//license = fmt.Sprintf("%v (%.0f%%)", blame.License.Name, 100*blame.License.Confidence)
				licenseConfidence = blame.License.Confidence
			}
			lines := []sourcecode.BlameLines{}
			var sloc, loc, comments, blanks int64
			for _, line := range blame.Lines {
				lines = append(lines, sourcecode.BlameLines{
					Sha:         line.SHA,
					AuthorRefID: line.Email,
					Date:        line.Date.Format("2006-01-02T15:04:05.000000Z-07:00"),
					Comment:     line.Comment,
					Code:        line.Code,
					Blank:       line.Blank,
				})
				loc++
				if line.Code {
					sloc++ // safety check below
				}
				if line.Comment {
					comments++
				}
				if line.Blank {
					blanks++
				}
			} // safety check
			if blame.Sloc != sloc {
				panic("logic problem: sloc didn't match")
			}

			commitCommentsCount += comments
			commitSlocCount += sloc
			commitLocCount += loc
			commitBlanksCount += blanks

			cf := commit.Files[blame.Filename]
			if blame.Language == "" {
				blame.Language = unknownLanguage
			}
			excluded := blame.Skipped != ""

			if excluded {
				excludedFilesCount++
			}
			commitAdditions += int64(cf.Additions)
			commitDeletions += int64(cf.Deletions)
			var lic string
			if blame.License != nil {
				lic = blame.License.Name
			}

			{
				cf := sourcecode.CommitFiles{
					CommitID:          s.commitID(commit.SHA),
					RepoID:            repoID,
					Status:            string(cf.Status),
					Ordinal:           ordinal,
					Filename:          cf.Filename,
					Language:          blame.Language,
					Renamed:           cf.Renamed,
					RenamedFrom:       cf.RenamedFrom,
					RenamedTo:         cf.RenamedTo,
					Additions:         int64(cf.Additions),
					Deletions:         int64(cf.Deletions),
					Binary:            cf.Binary,
					Excluded:          blame.Skipped != "",
					ExcludedReason:    blame.Skipped,
					License:           lic,
					LicenseConfidence: float64(licenseConfidence),
					Size:              blame.Size,
					Loc:               blame.Loc,
					Sloc:              blame.Sloc,
					Comments:          blame.Comments,
					Blanks:            blame.Blanks,
					Complexity:        blame.Complexity,
				}
				date.ConvertToModel(commit.Date, &cf.CreatedDate)
				commitFiles = append(commitFiles, cf)
			}

			commitComplexityCount += blame.Complexity
			commitSize += blame.Size
			commitFilesCount++
			// if exclude but not deleted, we don't need to write to commit activity
			// we need to write to commit_activity for deleted so we can track the last
			// deleted commit so sloc will add correctly to reflect the deleted sloc
			if excluded && cf.Status != ripsrc.GitFileCommitStatusRemoved {
				continue
			}

			bl := sourcecode.Blame{
				Status:         statusFromRipsrc(blame.Status),
				License:        &lic,
				Excluded:       blame.Skipped != "",
				ExcludedReason: blame.Skipped,
				CommitID:       s.commitID(commit.SHA),
				RefID:          commit.SHA,
				RefType:        s.opts.RefType,
				CustomerID:     customerID,
				Hashcode:       "",
				Size:           blame.Size,
				Loc:            int64(loc),
				Sloc:           int64(sloc),
				Blanks:         int64(blanks),
				Comments:       int64(comments),
				Filename:       blame.Filename,
				Language:       blame.Language,
				Sha:            commit.SHA,
				RepoID:         repoID,
				Complexity:     blame.Complexity,
				Lines:          lines,
			}
			date.ConvertToModel(commit.Date, &bl.ChangeDate)

			err := writeBlame(bl)
			if err != nil {
				return "", err
			}
			ordinal++
		}

		c := sourcecode.Commit{
			RefID:      commit.SHA,
			RefType:    s.opts.RefType,
			CustomerID: customerID,
			RepoID:     repoID,
			Sha:        commit.SHA,
			Message:    commit.Message,
			URL:        commitURL(s.opts.CommitURLTemplate, commit.SHA),
			//Branch:         branch, // TODO: remove this from datamodel
			Additions:      commitAdditions,
			Deletions:      commitDeletions,
			FilesChanged:   commitFilesCount,
			AuthorRefID:    ids.CodeCommitEmail(customerID, commit.AuthorEmail),
			CommitterRefID: ids.CodeCommitEmail(customerID, commit.CommitterEmail),
			Ordinal:        commit.Ordinal,
			Loc:            commitLocCount,
			Sloc:           commitSlocCount,
			Comments:       commitCommentsCount,
			Blanks:         commitBlanksCount,
			Size:           commitSize,
			Complexity:     commitComplexityCount,
			GpgSigned:      commit.Signed,
			Excluded:       excludedFilesCount == commitFilesCount,
			Files:          commitFiles,
		}

		date.ConvertToModel(commit.Date, &c.CreatedDate)

		err := writeCommit(c)
		if err != nil {
			return "", err
		}

	}

	return
}

const (
	unknownUser     = "unknown-deleter"
	unknownLanguage = "unknown"
)

func statusFromRipsrc(status ripsrc.CommitStatus) sourcecode.BlameStatus {
	switch status {
	case ripsrc.GitFileCommitStatusAdded:
		return sourcecode.BlameStatusAdded
	case ripsrc.GitFileCommitStatusModified:
		return sourcecode.BlameStatusModified
	case ripsrc.GitFileCommitStatusRemoved:
		return sourcecode.BlameStatusRemoved
	}
	return 0
}

func commitURL(commitURLTemplate, sha string) string {
	return strings.ReplaceAll(commitURLTemplate, "@@@sha@@@", sha)
}

func branchURL(branchURLTemplate, branchName string) string {
	return strings.ReplaceAll(branchURLTemplate, "@@@branch@@@", branchName)
}
