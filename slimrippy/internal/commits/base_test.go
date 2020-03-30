package commits

import (
	"testing"

	"github.com/pinpt/agent/slimrippy/testutil"
	"github.com/stretchr/testify/assert"
)

type Test struct {
	t        *testing.T
	repoName string
	tempDir  string
}

func NewTest(t *testing.T, repoName string) *Test {
	s := &Test{}
	s.t = t
	s.repoName = repoName
	return s
}

// cb callback to defer dirs.Remove()
func (s *Test) Run(cb func(opts Opts)) {
	dirs := testutil.UnzipTestRepo(s.repoName)
	defer dirs.Remove()

	opts := Opts{}
	opts.RepoDir = dirs.RepoDir
	cb(opts)
}

func assertResult(t *testing.T, want, got []Commit) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("invalid number of commits, wanted %v, got %v", len(want), len(got))
	}
	for i := range want {
		if want[i].SHA != got[i].SHA {
			t.Fatalf("invalid commit at position %v, wanted %v, got %v", i, want[i].SHA, got[i].SHA)
			continue
		}
		if !assert.Equal(t, want[i], got[i]) {
			t.Fail()
		}
	}
}
