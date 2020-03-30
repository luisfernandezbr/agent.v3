package e2etests

import (
	"context"
	"testing"

	"github.com/hashicorp/go-hclog"

	"gopkg.in/src-d/go-git.v4/plumbing/object"

	"github.com/stretchr/testify/assert"

	"github.com/pinpt/agent/slimrippy/internal/branches"
	"github.com/pinpt/agent/slimrippy/internal/commits"
	"github.com/pinpt/agent/slimrippy/internal/parentsgraph"
	"github.com/pinpt/agent/slimrippy/testutil"
)

type Test struct {
	t        *testing.T
	repoName string
	opts     *branches.Opts
}

func NewTest(t *testing.T, repoName string, opts *branches.Opts) *Test {
	s := &Test{}
	s.t = t
	s.repoName = repoName
	s.opts = opts
	return s
}

func (s *Test) Run() []branches.Branch {
	return s.run()
}

func (s *Test) run() []branches.Branch {
	t := s.t
	dirs := testutil.UnzipTestRepo(s.repoName)
	defer dirs.Remove()

	ctx := context.Background()
	commitsChan := make(chan *object.Commit, 1000) // enough size to process everything in tests
	_, err := commits.Commits(ctx, commits.Opts{RepoDir: dirs.RepoDir}, commitsChan)
	if err != nil {
		panic(err)
	}
	var commits []*object.Commit
	for c := range commitsChan {
		commits = append(commits, c)
	}
	pg, _ := parentsgraph.New(parentsgraph.Opts{
		Commits: commitsChanFromSlice(commits),
	})
	opts := branches.Opts{}
	if s.opts != nil {
		opts = *s.opts
	}
	opts.RepoDir = dirs.RepoDir
	opts.CommitGraph = pg
	opts.Logger = hclog.New(hclog.DefaultOptions)
	b := branches.New(opts)
	res, err := b.RunSlice(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func commitsChanFromSlice(commits []*object.Commit) chan *object.Commit {
	res := make(chan *object.Commit)
	go func() {
		for _, c := range commits {
			res <- c
		}
		close(res)
	}()
	return res
}

func assertResult(t *testing.T, want, got []branches.Branch) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("invalid result count, wanted %v, got %v", len(want), len(got))
	}
	gotCopy := make([]branches.Branch, len(got))
	copy(gotCopy, got)

	for i := range want {
		g := gotCopy[i]
		g.BranchID = "" // do not compare id
		assert.Equal(t, want[i], g)
	}
}
