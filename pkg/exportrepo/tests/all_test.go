package tests

import (
	"testing"
	"time"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/exportrepo"

	"github.com/pinpt/integration-sdk/sourcecode"
)

func CommitCreatedDate(s string) (res sourcecode.CommitCreatedDate) {
	d, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	date.ConvertToModel(d, &res)
	return
}

func strp(v string) *string {
	return &v
}

func TestBasic1(t *testing.T) {
	want := map[string]interface{}{}

	want["sourcecode.Commit"] = []sourcecode.Commit{
		{
			AuthorRefID:    "562d0daa5e0b4946",
			CommitterRefID: "562d0daa5e0b4946",
			CreatedDate:    CommitCreatedDate("2019-02-07T20:17:18+01:00"),
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
			CreatedDate:    CommitCreatedDate("2019-02-07T20:17:34+01:00"),
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

func TestPullRequestBranches(t *testing.T) {
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
			CreatedDate:    CommitCreatedDate("2019-02-07T20:17:18+01:00"),
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
			CreatedDate:    CommitCreatedDate("2019-02-07T20:17:34+01:00"),
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
