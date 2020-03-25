package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/go-hclog"

	"github.com/pinpt/agent/cmd/cmdexport/process"
	"github.com/pinpt/agent/pkg/exportrepo"
	"github.com/pinpt/agent/pkg/expsessions"
	"github.com/pinpt/agent/pkg/fsconf"
	"github.com/pinpt/agent/pkg/jsonstore"
	"github.com/pinpt/agent/slimrippy/pkg/testutil"
	"github.com/pinpt/integration-sdk/sourcecode"
)

type Test struct {
	t        *testing.T
	repoName string
	opts     *exportrepo.Opts
}

func NewTest(t *testing.T, repoName string, opts *exportrepo.Opts) *Test {
	s := &Test{}
	s.t = t
	s.repoName = repoName
	s.opts = opts
	return s
}

func (s *Test) Run(want map[string]interface{}) {
	dirs := testutil.UnzipTestRepo(s.repoName)
	defer dirs.Remove()

	locs := fsconf.New(filepath.Join(dirs.TempWrapper, "pproot"))

	lastProcessed, err := jsonstore.New(locs.LastProcessedFile)
	if err != nil {
		panic(err)
	}

	logger := hclog.New(hclog.DefaultOptions)
	ctx := context.Background()

	mockWriters := expsessions.NewMockWriters()

	sessions := expsessions.New(expsessions.Opts{
		Logger:        logger,
		LastProcessed: lastProcessed,
		NewWriter:     mockWriters.NewWriter,
	})

	opts := exportrepo.Opts{}
	if s.opts != nil {
		opts = *s.opts
	}
	opts.Logger = logger
	opts.LocalRepo = dirs.RepoDir
	opts.Sessions = sessions
	opts.RepoID = "r1"
	opts.UniqueName = s.repoName
	opts.CustomerID = "c1"
	opts.LastProcessed = lastProcessed
	opts.CommitURLTemplate = "/commit/@@@sha@@@"
	opts.BranchURLTemplate = "/branch/@@@branch@@@"
	opts.RefType = "git"
	opts.CommitUsers = process.NewCommitUsers()

	exp := exportrepo.New(opts, locs)
	res := exp.Run(ctx)

	t := s.t
	if res.SessionErr != nil {
		t.Fatal(fmt.Errorf("export failed session err: %v", res.SessionErr))
	}
	if res.OtherErr != nil {
		t.Fatal(fmt.Errorf("export failed not session err (other err): %v", res.OtherErr))
	}
	if err := lastProcessed.Save(); err != nil {
		t.Fatal(err)
	}

	got := mockWriters.Data()
	wantConverted := map[string][]map[string]interface{}{}
	for model, data := range want {
		var data2 []Model
		switch data := data.(type) {
		case []sourcecode.Branch:
			for _, obj := range data {
				obj2 := obj
				data2 = append(data2, &obj2)
			}
		case []sourcecode.Commit:
			for _, obj := range data {
				obj2 := obj
				data2 = append(data2, &obj2)
			}
		case []sourcecode.User:
			for _, obj := range data {
				obj2 := obj
				data2 = append(data2, &obj2)
			}
		case []sourcecode.PullRequestBranch:
			for _, obj := range data {
				obj2 := obj
				data2 = append(data2, &obj2)
			}
		default:
			panic("unknown record type")
		}
		for _, obj := range data2 {
			wantConverted[model] = append(wantConverted[model], obj.(Model).ToMap())
		}
	}
	keys := []string{"sourcecode.Commit", "sourcecode.CommitUser", "sourcecode.Branch", "sourcecode.PullRequestBranch"}
	for _, k := range keys {
		want := wantConverted[k]
		sort.Slice(want, func(i, j int) bool {
			return want[i]["id"].(string) < want[j]["id"].(string)
		})
		got := got[k]
		if len(want) != len(got) {
			t.Fatalf("got invalid number of %v, wanted %v, got %v", k, len(want), len(got))
		}
		for i := range want {
			wantObj := want[i]
			gotObj := got[i]
			if !reflect.DeepEqual(gotObj, wantObj) {
				t.Error("wanted ref_ids")
				for _, obj := range want {
					t.Error(obj["ref_id"])
				}
				t.Error("got ref_ids")
				for _, obj := range got {
					t.Error(obj["ref_id"])
				}
				t.Fatalf("wanted object %v\n%v\ngot\n%v", k, pretty(wantObj), pretty(gotObj))
			}
		}
	}
}

func pretty(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(b)
}

type Model interface {
	GetID() string
	ToMap() map[string]interface{}
}

/*
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
*/
