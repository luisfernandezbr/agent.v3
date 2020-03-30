package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/pinpt/agent/cmd/cmdexport/process"
	"github.com/pinpt/agent/pkg/expsessions"
	"github.com/pinpt/agent/pkg/fsconf"
	"github.com/pinpt/agent/pkg/jsonstore"
	"github.com/pinpt/agent/slimrippy/exportrepo"
	"github.com/pinpt/agent/slimrippy/testutil"
	"github.com/pinpt/integration-sdk/sourcecode"
)

type Opts struct {
	T        *testing.T
	RepoName string
	Export   *exportrepo.Opts
	Want     map[string]interface{}

	IncrementalStep1 bool
	Dirs             *TestDirs
}

func Run(opts Opts) *TestDirs {
	t := opts.T
	repoName := opts.RepoName
	want := opts.Want

	repo := testutil.UnzipTestRepo(repoName)
	defer repo.Remove()

	var testDirs TestDirs
	if opts.Dirs != nil {
		testDirs = *opts.Dirs
	} else {
		testDirs = NewTestDirs()
	}

	locs := fsconf.New(testDirs.PPRoot)

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

	eo := exportrepo.Opts{}
	if opts.Export != nil {
		eo = *opts.Export
	}
	eo.Logger = logger
	eo.LocalRepo = repo.RepoDir
	eo.Sessions = sessions
	eo.RepoID = "r1"
	eo.UniqueName = repoName
	eo.CustomerID = "c1"
	eo.LastProcessed = lastProcessed
	eo.CommitURLTemplate = "/commit/@@@sha@@@"
	eo.BranchURLTemplate = "/branch/@@@branch@@@"
	eo.RefType = "git"
	eo.CommitUsers = process.NewCommitUsers()

	exp := exportrepo.New(eo, locs)
	res := exp.Run(ctx)

	if res.SessionErr != nil {
		t.Fatal(fmt.Errorf("export failed session err: %v", res.SessionErr))
	}
	if res.OtherErr != nil {
		t.Fatal(fmt.Errorf("export failed not session err (other err): %v", res.OtherErr))
	}
	if err := lastProcessed.Save(); err != nil {
		t.Fatal(err)
	}

	if opts.IncrementalStep1 {
		return &testDirs
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
				t.Error("failed on", k)
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
	return nil
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

func parseGitDate(s string) time.Time {
	//Tue Nov 27 21:55:36 2018 +0100
	r, err := time.Parse("Mon Jan 2 15:04:05 2006 -0700", s)
	if err != nil {
		panic(err)
	}
	return r
}

type TestDirs struct {
	Wrapper string
	PPRoot  string
}

func NewTestDirs() TestDirs {
	tempDir, err := ioutil.TempDir("", "ripsrc-test-")
	if err != nil {
		panic(err)
	}
	return TestDirs{
		Wrapper: tempDir,
		PPRoot:  filepath.Join(tempDir, "pp-root"),
	}
}

func (s TestDirs) Remove() {
	err := os.RemoveAll(s.Wrapper)
	if err != nil {
		panic(err)
	}
}
