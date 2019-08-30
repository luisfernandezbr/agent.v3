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
	"github.com/pinpt/go-common/hash"

	"github.com/pinpt/integration-sdk/sourcecode"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/agent.next/pkg/gitclone"
	"github.com/pinpt/agent.next/pkg/ids"
	"github.com/pinpt/agent.next/pkg/jsonstore"
	"github.com/pinpt/agent.next/pkg/outsession"
	"github.com/pinpt/ripsrc/ripsrc"

	"github.com/hashicorp/go-hclog"
)

type Opts struct {
	Logger     hclog.Logger
	CustomerID string
	RepoID     string
	Sessions   *outsession.Manager

	LastProcessed *jsonstore.Store
	RepoAccess    gitclone.AccessDetails

	// CommitURLTemplate is a template for building commit url
	// https://example.com/repo1/@@@sha@@@
	CommitURLTemplate string
}

type Export struct {
	opts   Opts
	locs   fsconf.Locs
	logger hclog.Logger
	//defaultBranch string

	repoNameUsedInCacheDir string
	lastProcessedKey       []string

	rip *ripsrc.Ripsrc

	// TODO: pass from options
	refType string
}

func New(opts Opts, locs fsconf.Locs) *Export {
	if opts.CustomerID == "" {
		panic("provide CustomerID")
	}
	if opts.RepoID == "" {
		panic("provide RepoID")
	}
	s := &Export{}
	s.opts = opts
	s.logger = opts.Logger.Named("exportrepo")
	s.locs = locs
	s.refType = "github"
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

	tempDir, err := ioutil.TempDir(s.locs.Temp, "")
	if err != nil {
		return "", "", err
	}

	dirs := gitclone.Dirs{
		CacheRoot: s.locs.RepoCache,
		Checkout:  tempDir,
	}
	res, err := gitclone.CloneWithCache(ctx, s.logger, s.opts.RepoAccess, dirs)

	if err != nil {
		return "", "", err
	}

	//s.defaultBranch = res.DefaultBranch

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
	sessionID, _, err := sessions.NewSession(sourcecode.BranchModelName.String())
	if err != nil {
		return err
	}
	defer sessions.Done(sessionID, nil)

	res := make(chan ripsrc.Branch)
	done := make(chan bool)

	go func() {
		for data := range res {
			obj := sourcecode.Branch{}
			obj.RefID = data.Name
			obj.RefType = s.refType
			obj.CustomerID = s.opts.CustomerID
			obj.Name = data.Name
			obj.Default = data.IsDefault
			obj.Merged = data.IsMerged
			obj.MergeCommitID = data.MergeCommit
			obj.BranchedFromCommitIds = data.BranchedFromCommits
			for _, sha := range data.Commits {
				id := ids.CodeCommit(s.opts.CustomerID, s.refType, sha)
				obj.CommitIds = append(obj.CommitIds, id)
			}
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
	blameSession, _, err := sessions.NewSession(sourcecode.BlameModelName.String())
	if err != nil {
		return "", err
	}
	commitSession, _, err := sessions.NewSession(sourcecode.CommitModelName.String())
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
		var lastBlame *ripsrc.BlameResult
		commitFiles := []sourcecode.CommitFiles{}
		var excludedFilesCount int64
		for blame := range commit.Files {
			lastBlame = &blame
			if blame.Commit.SHA == "" {
				panic(`blame.Commit.SHA == ""`)
			}
			//var license string
			var licenseConfidence float32
			if blame.License != nil {
				//license = fmt.Sprintf("%v (%.0f%%)", blame.License.Name, 100*blame.License.Confidence)
				licenseConfidence = blame.License.Confidence
			}
			//s.logger.Debug(fmt.Sprintf("[%s] %s language=%s,license=%v,loc=%v,sloc=%v,comments=%v,blanks=%v,complexity=%v,skipped=%v", blame.Commit.SHA[0:8], blame.Filename, blame.Language, license, blame.Loc, blame.Sloc, blame.Comments, blame.Comments, blame.Complexity, blame.Skipped))
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

			cf := blame.Commit.Files[blame.Filename]
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
					CommitID:          ids.CodeCommit(customerID, s.refType, commit.SHA),
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
				date.ConvertToModel(blame.Commit.Date, &cf.CreatedDate)
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
				CommitID:       ids.CodeCommit(customerID, s.refType, blame.Commit.SHA),
				RefID:          blame.Commit.SHA,
				RefType:        s.refType,
				CustomerID:     customerID,
				Hashcode:       "",
				Size:           blame.Size,
				Loc:            int64(loc),
				Sloc:           int64(sloc),
				Blanks:         int64(blanks),
				Comments:       int64(comments),
				Filename:       blame.Filename,
				Language:       blame.Language,
				Sha:            blame.Commit.SHA,
				RepoID:         repoID,
				Complexity:     blame.Complexity,
				Lines:          lines,
			}
			date.ConvertToModel(blame.Commit.Date, &bl.ChangeDate)

			err := writeBlame(bl)
			if err != nil {
				return "", err
			}
			ordinal++
		}

		if lastBlame != nil {
			c := sourcecode.Commit{
				RefID:      commit.SHA,
				RefType:    s.refType,
				CustomerID: customerID,
				Hashcode:   "",
				RepoID:     repoID,
				Sha:        commit.SHA,
				Message:    lastBlame.Commit.Message,
				URL:        buildURL(s.opts.CommitURLTemplate, commit.SHA),
				//Branch:         branch, // TODO: remove this from datamodel
				Additions:      commitAdditions,
				Deletions:      commitDeletions,
				FilesChanged:   commitFilesCount,
				AuthorRefID:    hash.Values(customerID, lastBlame.Commit.AuthorEmail),
				CommitterRefID: hash.Values(customerID, lastBlame.Commit.CommitterEmail),
				Ordinal:        lastBlame.Commit.Ordinal,
				Loc:            commitLocCount,
				Sloc:           commitSlocCount,
				Comments:       commitCommentsCount,
				Blanks:         commitBlanksCount,
				Size:           commitSize,
				Complexity:     commitComplexityCount,
				GpgSigned:      lastBlame.Commit.Signed,
				Excluded:       excludedFilesCount == commitFilesCount,
				Files:          commitFiles,
			}
			date.ConvertToModel(lastBlame.Commit.Date, &c.CreatedDate)

			err := writeCommit(c)
			if err != nil {
				return "", err
			}
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

func buildURL(commitURLTemplate, sha string) string {
	return strings.ReplaceAll(commitURLTemplate, "@@@sha@@@", sha)
}
