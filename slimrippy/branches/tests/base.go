package e2etests

import (
	"context"
	"testing"

	"github.com/hashicorp/go-hclog"

	"github.com/stretchr/testify/assert"

	"github.com/pinpt/agent/slimrippy/branches"
	"github.com/pinpt/agent/slimrippy/pkg/testutil"
	"github.com/pinpt/agent/slimrippy/slimrippy"
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
	repoDir := dirs.RepoDir

	res := []branches.Branch{}
	opts := slimrippy.Opts{}
	opts.Logger = hclog.NewNullLogger()
	opts.RepoDir = repoDir
	if s.opts != nil {
		opts.BranchOpts = s.opts
	} else {
		opts.BranchOpts = &branches.Opts{}
	}
	opts.BranchCallback = func(b branches.Branch) error {
		res = append(res, b)
		return nil
	}
	_, err := slimrippy.CommitsAndBranches(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
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
