package api

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/integration-sdk/sourcecode"
	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/go-hclog"
)

var conf Creds

func skipTests(t *testing.T) bool {
	if os.Getenv("PP_TEST_TFS_CODE") == "" {
		t.Skip("skipping sonarqube tests")
		return true
	}
	conf = Creds{
		URL:        os.Getenv("PP_TEST_TFS_CODE_URL"),
		Collection: "DefaultCollection",
		APIKey:     os.Getenv("PP_TEST_TFS_CODE_APIKEY"),
	}
	if conf.URL == "" {
		panic("missing PP_TEST_TFS_CODE_URL")
	}
	if conf.APIKey == "" {
		panic("missing APIKey")
	}
	return false
}

func TestFetchRepos(t *testing.T) {
	if skipTests(t) {
		return
	}
	a := NewTFSAPI(context.Background(), hclog.NewNullLogger(), "1234567890", "tfs", &conf)
	repos, err := a.FetchRepos([]string{}, []string{})
	assert.NoError(t, err)
	assert.NotEmpty(t, repos)
}

func TestFetchCommitUsers(t *testing.T) {
	if skipTests(t) {
		return
	}
	a := NewTFSAPI(context.Background(), hclog.NewNullLogger(), "1234567890", "tfs", &conf)
	repos, err := a.FetchRepos([]string{}, []string{})
	assert.NoError(t, err)
	for _, repo := range repos {
		a := NewTFSAPI(context.Background(), hclog.NewNullLogger(), "1234567890", "tfs", &conf)
		usermap := make(map[string]*sourcecode.User)
		err := a.FetchCommitUsers(repo.RefID, usermap, time.Time{})
		assert.NoError(t, err)
	}
}

func TestFetchPullRequests(t *testing.T) {
	if skipTests(t) {
		return
	}
	a := NewTFSAPI(context.Background(), hclog.NewNullLogger(), "1234567890", "tfs", &conf)
	a.RepoID = func(refid string) string {
		return hash.Values("Repo", "customer_id", "tfs", refid)
	}
	a.BranchID = func(repoRefID string, branchName string) string {
		repoID := a.RepoID(repoRefID)
		return hash.Values("tfs", repoID, "1234567890", branchName)
	}
	a.PullRequestID = func(refID string) string {
		return hash.Values("PullRequest", "1234567890", "tfs", refID)
	}
	repos, err := a.FetchRepos([]string{}, []string{})
	assert.NoError(t, err)
	for _, repo := range repos {
		if strings.Contains(repo.Name, "agent") {
			_, _, err := a.FetchPullRequests(repo.RefID, time.Time{})
			assert.NoError(t, err)
			// check any PRs created after now, should be 0
			prs, _, err := a.FetchPullRequests(repo.RefID, time.Now())
			assert.Len(t, prs, 0)
		}
	}
}
func TestFetchPullRequestComments(t *testing.T) {
	if skipTests(t) {
		return
	}
	a := NewTFSAPI(context.Background(), hclog.NewNullLogger(), "1234567890", "tfs", &conf)
	a.RepoID = func(refid string) string {
		return hash.Values("Repo", "customer_id", "tfs", refid)
	}
	a.BranchID = func(repoRefID string, branchName string) string {
		repoID := a.RepoID(repoRefID)
		return hash.Values("tfs", repoID, "1234567890", branchName)
	}
	a.PullRequestID = func(refID string) string {
		return hash.Values("PullRequest", "1234567890", "tfs", refID)
	}
	repos, err := a.FetchRepos([]string{}, []string{})
	assert.NoError(t, err)
	for _, repo := range repos {
		prs, _, err := a.FetchPullRequests(repo.RefID, time.Time{})
		assert.NoError(t, err)
		for _, p := range prs {
			_, err := a.FetchPullRequestComments(repo.RefID, p.RefID)
			assert.NoError(t, err)
		}
	}

}
