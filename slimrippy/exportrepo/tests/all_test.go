package tests

import (
	"testing"
	"time"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/slimrippy/exportrepo"

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

	opts := Opts{}
	opts.T = t
	opts.RepoName = "basic1"
	opts.Want = want
	Run(opts)
}

func TestExportRepoPullRequestBranches(t *testing.T) {
	pr1 := exportrepo.PR{
		ID:            "prid",
		RefID:         "prrefid",
		BranchName:    "branchname",
		URL:           "/pr/1",
		LastCommitSHA: "9b39087654af70197f68d0b3d196a4a20d987cd6",
	}
	exportOpts := &exportrepo.Opts{}
	exportOpts.PRs = []exportrepo.PR{pr1}

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

	opts := Opts{}
	opts.T = t
	opts.RepoName = "basic1"
	opts.Want = want
	opts.Export = exportOpts
	Run(opts)
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

	opts := Opts{}
	opts.T = t
	opts.RepoName = "remote-has-remote"
	opts.Want = want
	Run(opts)
}

func TestExportRepoIncremental1(t *testing.T) {
	opts := Opts{}
	opts.T = t
	opts.RepoName = "basic1"
	opts.IncrementalStep1 = true
	dirs := Run(opts)

	want := map[string]interface{}{}

	want["sourcecode.Commit"] = []sourcecode.Commit{
		{
			AuthorRefID:    "562d0daa5e0b4946",
			CommitterRefID: "562d0daa5e0b4946",
			CreatedDate:    CommitCreatedDate(parseGitDate("Fri Mar 27 14:21:45 2020 +0100")),
			CustomerID:     "c1",
			Message:        "c3",
			RefID:          "63b0ac79015985fe248ba0ea3e34fa464fae1b7a",
			RefType:        "git",
			RepoID:         "r1",
			Sha:            "63b0ac79015985fe248ba0ea3e34fa464fae1b7a",
			URL:            "/commit/63b0ac79015985fe248ba0ea3e34fa464fae1b7a",
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
			AheadDefaultCount:     2,
			BehindDefaultCount:    0,
			BranchedFromCommitIds: []string{"c9074ba50d54337b"},
			BranchedFromCommitShas: []string{
				"33e223d1fd8393dc98596727d370e51e7b3b7fba"},
			CommitIds: []string{
				"29cc1d6ed7f46dfc",
				"8eeb6347f41c9190"},
			CommitShas: []string{
				"9b39087654af70197f68d0b3d196a4a20d987cd6",
				"63b0ac79015985fe248ba0ea3e34fa464fae1b7a"},
			CustomerID:     "c1",
			Default:        false,
			FirstCommitID:  "29cc1d6ed7f46dfc",
			FirstCommitSha: "9b39087654af70197f68d0b3d196a4a20d987cd6",
			MergeCommitID:  "",
			MergeCommitSha: "",
			Merged:         false,
			Name:           "a",
			RefID:          "a",
			RefType:        "git",
			RepoID:         "r1",
			URL:            "/branch/a",
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

	opts = Opts{}
	opts.T = t
	opts.RepoName = "basic-incremental1"
	opts.Dirs = dirs
	opts.Want = want

	Run(opts)
}

func TestExportRepoIncrementalPullRequests1(t *testing.T) {
	opts := Opts{}
	opts.T = t
	opts.RepoName = "basic1"
	opts.IncrementalStep1 = true
	dirs := Run(opts)

	// it is possible to pass prs that point to any commit, they should be processed properly in incremental
	pr1 := exportrepo.PR{
		ID:            "prid",
		RefID:         "prrefid",
		BranchName:    "branchname",
		URL:           "/pr/1",
		LastCommitSHA: "9b39087654af70197f68d0b3d196a4a20d987cd6",
	}
	exportOpts := &exportrepo.Opts{}
	exportOpts.PRs = []exportrepo.PR{pr1}

	want := map[string]interface{}{}

	want["sourcecode.Commit"] = []sourcecode.Commit{
		{
			AuthorRefID:    "562d0daa5e0b4946",
			CommitterRefID: "562d0daa5e0b4946",
			CreatedDate:    CommitCreatedDate(parseGitDate("Fri Mar 27 14:21:45 2020 +0100")),
			CustomerID:     "c1",
			Message:        "c3",
			RefID:          "63b0ac79015985fe248ba0ea3e34fa464fae1b7a",
			RefType:        "git",
			RepoID:         "r1",
			Sha:            "63b0ac79015985fe248ba0ea3e34fa464fae1b7a",
			URL:            "/commit/63b0ac79015985fe248ba0ea3e34fa464fae1b7a",
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
			AheadDefaultCount:     2,
			BehindDefaultCount:    0,
			BranchedFromCommitIds: []string{"c9074ba50d54337b"},
			BranchedFromCommitShas: []string{
				"33e223d1fd8393dc98596727d370e51e7b3b7fba"},
			CommitIds: []string{
				"29cc1d6ed7f46dfc",
				"8eeb6347f41c9190"},
			CommitShas: []string{
				"9b39087654af70197f68d0b3d196a4a20d987cd6",
				"63b0ac79015985fe248ba0ea3e34fa464fae1b7a"},
			CustomerID:     "c1",
			Default:        false,
			FirstCommitID:  "29cc1d6ed7f46dfc",
			FirstCommitSha: "9b39087654af70197f68d0b3d196a4a20d987cd6",
			MergeCommitID:  "",
			MergeCommitSha: "",
			Merged:         false,
			Name:           "a",
			RefID:          "a",
			RefType:        "git",
			RepoID:         "r1",
			URL:            "/branch/a",
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

	opts = Opts{}
	opts.T = t
	opts.RepoName = "basic-incremental1"
	opts.Dirs = dirs
	opts.Want = want
	opts.Export = exportOpts

	Run(opts)
}
