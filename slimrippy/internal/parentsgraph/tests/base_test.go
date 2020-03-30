package tests

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/pinpt/agent/slimrippy/internal/commits"
	"github.com/pinpt/agent/slimrippy/internal/parentsgraph"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	"github.com/pinpt/agent/slimrippy/testutil"
)

type Test struct {
	t        *testing.T
	repoName string
	opts     *parentsgraph.Opts
}

func NewTest(t *testing.T, repoName string, opts *parentsgraph.Opts) *Test {
	s := &Test{}
	s.t = t
	s.repoName = repoName
	s.opts = opts
	return s
}

func (s *Test) Run() *parentsgraph.Graph {
	dirs := testutil.UnzipTestRepo(s.repoName)
	defer dirs.Remove()

	ctx := context.Background()
	commitsChan := make(chan *object.Commit, 1000) // enough size to process everything in tests
	_, err := commits.Commits(ctx, commits.Opts{RepoDir: dirs.RepoDir}, commitsChan)
	if err != nil {
		panic(err)
	}
	opts := s.opts
	if opts == nil {
		opts = &parentsgraph.Opts{}
	}
	opts.Commits = commitsChan
	pg, _ := parentsgraph.New(*opts)
	return pg
}

func assertResult(t *testing.T, pg *parentsgraph.Graph, wantParents, wantChildren map[string][]string) {
	t.Helper()
	assertEqualMaps(t, wantParents, pg.Parents, "parents")
	assertEqualMaps(t, wantChildren, pg.Children, "children")
}

func assertEqualMaps(t *testing.T, wantMap, gotMap map[string][]string, label string) {
	t.Helper()
	for _, data := range wantMap {
		sort.Strings(data)
	}
	if !reflect.DeepEqual(wantMap, gotMap) {
		t.Errorf("invalid map %v\ngot\n%v\nwanted\n%v", label, gotMap, wantMap)
	}
}
