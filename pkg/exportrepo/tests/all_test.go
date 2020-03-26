package tests

import (
	"testing"
	"time"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/exportrepo"

	"github.com/pinpt/integration-sdk/sourcecode"
)

func CommitCreatedDateStr(s string) (res sourcecode.CommitCreatedDate) {
	d, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	date.ConvertToModel(d, &res)
	return
}

func CommitCreatedDate(d time.Time) (res sourcecode.CommitCreatedDate) {
	date.ConvertToModel(d, &res)
	return
}

func strp(v string) *string {
	return &v
}

func TestExportRepoBasic1(t *testing.T) {
	want := map[string]interface{}{}

	want["sourcecode.Commit"] = []sourcecode.Commit{
		{
			AuthorRefID:    "562d0daa5e0b4946",
			CommitterRefID: "562d0daa5e0b4946",
			CreatedDate:    CommitCreatedDateStr("2019-02-07T20:17:18+01:00"),
			CustomerID:     "c1",
			Message:        "c1",
			RefID:          "33e223d1fd8393dc98596727d370e51e7b3b7fba",
			RefType:        "git",
			RepoID:         "r1",
			Sha:            "33e223d1fd8393dc98596727d370e51e7b3b7fba",
			URL:            "/commit/33e223d1fd8393dc98596727d370e51e7b3b7fba",
		},
		{
			AuthorRefID:    "562d0daa5e0b4946",
			CommitterRefID: "562d0daa5e0b4946",
			CreatedDate:    CommitCreatedDateStr("2019-02-07T20:17:34+01:00"),
			CustomerID:     "c1",
			Message:        "c2",
			RefID:          "9b39087654af70197f68d0b3d196a4a20d987cd6",
			RefType:        "git",
			RepoID:         "r1",
			Sha:            "9b39087654af70197f68d0b3d196a4a20d987cd6",
			URL:            "/commit/9b39087654af70197f68d0b3d196a4a20d987cd6",
		},
	}

	want["sourcecode.Branch"] = []sourcecode.Branch{
		{
			AheadDefaultCount:      0,
			BehindDefaultCount:     0,
			BranchedFromCommitIds:  nil,
			BranchedFromCommitShas: nil,
			CommitIds:              []string{"c9074ba50d54337b"},
			CommitShas:             []string{"33e223d1fd8393dc98596727d370e51e7b3b7fba"},
			CustomerID:             "c1",
			Default:                true,
			FirstCommitID:          "c9074ba50d54337b",
			FirstCommitSha:         "33e223d1fd8393dc98596727d370e51e7b3b7fba",
			MergeCommitID:          "",
			MergeCommitSha:         "",
			Merged:                 false,
			Name:                   "master",
			RefID:                  "master",
			RefType:                "git",
			RepoID:                 "r1",
			URL:                    "/branch/master",
		},
		{
			AheadDefaultCount:      1,
			BehindDefaultCount:     0,
			BranchedFromCommitIds:  []string{"c9074ba50d54337b"},
			BranchedFromCommitShas: []string{"33e223d1fd8393dc98596727d370e51e7b3b7fba"},
			CommitIds:              []string{"29cc1d6ed7f46dfc"},
			CommitShas:             []string{"9b39087654af70197f68d0b3d196a4a20d987cd6"},
			CustomerID:             "c1",
			Default:                false,
			FirstCommitID:          "29cc1d6ed7f46dfc",
			FirstCommitSha:         "9b39087654af70197f68d0b3d196a4a20d987cd6",
			MergeCommitID:          "",
			MergeCommitSha:         "",
			Merged:                 false,
			Name:                   "a",
			RefID:                  "a",
			RefType:                "git",
			RepoID:                 "r1",
			URL:                    "/branch/a",
		},
	}

	want["sourcecode.CommitUser"] = []sourcecode.User{
		{
			ID:         "3272aa1f3e23245f",
			RefID:      "562d0daa5e0b4946",
			RefType:    "git",
			CustomerID: "c1",
			Email:      strp("none"),
			Name:       "none",
		},
	}

	NewTest(t, "basic1", nil).Run(want)
}

func TestExportRepoPullRequestBranches(t *testing.T) {
	pr1 := exportrepo.PR{
		ID:            "prid",
		RefID:         "prrefid",
		BranchName:    "branchname",
		URL:           "/pr/1",
		LastCommitSHA: "9b39087654af70197f68d0b3d196a4a20d987cd6",
	}
	opts := &exportrepo.Opts{}
	opts.PRs = []exportrepo.PR{pr1}

	want := map[string]interface{}{}

	want["sourcecode.Commit"] = []sourcecode.Commit{
		{
			AuthorRefID:    "562d0daa5e0b4946",
			CommitterRefID: "562d0daa5e0b4946",
			CreatedDate:    CommitCreatedDateStr("2019-02-07T20:17:18+01:00"),
			CustomerID:     "c1",
			Message:        "c1",
			RefID:          "33e223d1fd8393dc98596727d370e51e7b3b7fba",
			RefType:        "git",
			RepoID:         "r1",
			Sha:            "33e223d1fd8393dc98596727d370e51e7b3b7fba",
			URL:            "/commit/33e223d1fd8393dc98596727d370e51e7b3b7fba",
		},
		{
			AuthorRefID:    "562d0daa5e0b4946",
			CommitterRefID: "562d0daa5e0b4946",
			CreatedDate:    CommitCreatedDateStr("2019-02-07T20:17:34+01:00"),
			CustomerID:     "c1",
			Message:        "c2",
			RefID:          "9b39087654af70197f68d0b3d196a4a20d987cd6",
			RefType:        "git",
			RepoID:         "r1",
			Sha:            "9b39087654af70197f68d0b3d196a4a20d987cd6",
			URL:            "/commit/9b39087654af70197f68d0b3d196a4a20d987cd6",
		},
	}

	want["sourcecode.Branch"] = []sourcecode.Branch{
		{
			AheadDefaultCount:      0,
			BehindDefaultCount:     0,
			BranchedFromCommitIds:  nil,
			BranchedFromCommitShas: nil,
			CommitIds:              []string{"c9074ba50d54337b"},
			CommitShas:             []string{"33e223d1fd8393dc98596727d370e51e7b3b7fba"},
			CustomerID:             "c1",
			Default:                true,
			FirstCommitID:          "c9074ba50d54337b",
			FirstCommitSha:         "33e223d1fd8393dc98596727d370e51e7b3b7fba",
			MergeCommitID:          "",
			MergeCommitSha:         "",
			Merged:                 false,
			Name:                   "master",
			RefID:                  "master",
			RefType:                "git",
			RepoID:                 "r1",
			URL:                    "/branch/master",
		},
		{
			AheadDefaultCount:      1,
			BehindDefaultCount:     0,
			BranchedFromCommitIds:  []string{"c9074ba50d54337b"},
			BranchedFromCommitShas: []string{"33e223d1fd8393dc98596727d370e51e7b3b7fba"},
			CommitIds:              []string{"29cc1d6ed7f46dfc"},
			CommitShas:             []string{"9b39087654af70197f68d0b3d196a4a20d987cd6"},
			CustomerID:             "c1",
			Default:                false,
			FirstCommitID:          "29cc1d6ed7f46dfc",
			FirstCommitSha:         "9b39087654af70197f68d0b3d196a4a20d987cd6",
			MergeCommitID:          "",
			MergeCommitSha:         "",
			Merged:                 false,
			Name:                   "a",
			RefID:                  "a",
			RefType:                "git",
			RepoID:                 "r1",
			URL:                    "/branch/a",
		},
	}

	want["sourcecode.PullRequestBranch"] = []sourcecode.PullRequestBranch{
		{
			AheadDefaultCount:      1,
			BehindDefaultCount:     0,
			BranchedFromCommitIds:  []string{"c9074ba50d54337b"},
			BranchedFromCommitShas: []string{"33e223d1fd8393dc98596727d370e51e7b3b7fba"},
			CommitIds:              []string{"29cc1d6ed7f46dfc"},
			CommitShas:             []string{"9b39087654af70197f68d0b3d196a4a20d987cd6"},
			CustomerID:             "c1",
			Default:                false,
			MergeCommitID:          "",
			MergeCommitSha:         "",
			Merged:                 false,
			Name:                   "branchname",
			PullRequestID:          pr1.ID,
			RefID:                  pr1.RefID,
			RefType:                "git",
			RepoID:                 "r1",
			URL:                    pr1.URL,
		},
	}

	want["sourcecode.CommitUser"] = []sourcecode.User{
		{
			ID:         "3272aa1f3e23245f",
			RefID:      "562d0daa5e0b4946",
			RefType:    "git",
			CustomerID: "c1",
			Email:      strp("none"),
			Name:       "none",
		},
	}

	NewTest(t, "basic1", opts).Run(want)
}

// The cloned repo here has remote set, make sure we don't export the remote branches of cloned repo
func TestExportRemoteHasRemote(t *testing.T) {
	want := map[string]interface{}{}

	want["sourcecode.Commit"] = []sourcecode.Commit{
		{
			AuthorRefID:    "562d0daa5e0b4946",
			CommitterRefID: "562d0daa5e0b4946",
			CreatedDate:    CommitCreatedDate(parseGitDate("Thu Mar 26 15:08:20 2020 +0100")),
			CustomerID:     "c1",
			Message:        "m1",
			RefID:          "63d8e58c077905aa51538184feb66852f02e2856",
			RefType:        "git",
			RepoID:         "r1",
			Sha:            "63d8e58c077905aa51538184feb66852f02e2856",
			URL:            "/commit/63d8e58c077905aa51538184feb66852f02e2856",
		},
		{
			AuthorRefID:    "562d0daa5e0b4946",
			CommitterRefID: "562d0daa5e0b4946",
			CreatedDate:    CommitCreatedDate(parseGitDate("Thu Mar 26 15:09:24 2020 +0100")),
			CustomerID:     "c1",
			Message:        "m2",
			RefID:          "0557506be087faa32994bf07ef7a559cf64123c9",
			RefType:        "git",
			RepoID:         "r1",
			Sha:            "0557506be087faa32994bf07ef7a559cf64123c9",
			URL:            "/commit/0557506be087faa32994bf07ef7a559cf64123c9",
		},
	}

	want["sourcecode.Branch"] = []sourcecode.Branch{
		{
			AheadDefaultCount:      0,
			BehindDefaultCount:     0,
			BranchedFromCommitIds:  nil,
			BranchedFromCommitShas: nil,
			CommitIds:              []string{"5686481d1ab7515a", "7eda1448ad486ee0"},
			CommitShas:             []string{"63d8e58c077905aa51538184feb66852f02e2856", "0557506be087faa32994bf07ef7a559cf64123c9"},
			CustomerID:             "c1",
			Default:                true,
			FirstCommitID:          "5686481d1ab7515a",
			FirstCommitSha:         "63d8e58c077905aa51538184feb66852f02e2856",
			MergeCommitID:          "",
			MergeCommitSha:         "",
			Merged:                 false,
			Name:                   "master",
			RefID:                  "master",
			RefType:                "git",
			RepoID:                 "r1",
			URL:                    "/branch/master",
		},
	}

	want["sourcecode.CommitUser"] = []sourcecode.User{
		{
			ID:         "3272aa1f3e23245f",
			RefID:      "562d0daa5e0b4946",
			RefType:    "git",
			CustomerID: "c1",
			Email:      strp("none"),
			Name:       "none",
		},
	}

	NewTest(t, "remote-has-remote", nil).Run(want)
}
