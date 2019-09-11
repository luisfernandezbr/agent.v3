package api

import (
	"context"
	"os"
	"testing"

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
	repos, projids, err := a.FetchRepos([]string{}, []string{})
	assert.NoError(t, err)
	assert.NotEmpty(t, repos)
	assert.NotEmpty(t, projids)
}

func TestFetchProjectUsers(t *testing.T) {
	if skipTests(t) {
		return
	}
	a := NewTFSAPI(context.Background(), hclog.NewNullLogger(), "1234567890", "tfs", &conf)
	_, projids, err := a.FetchRepos([]string{}, []string{})
	assert.NoError(t, err)
	for _, id := range projids {
		usermap := make(map[string]*sourcecode.User)
		err := a.FetchUsers(id, usermap)
		assert.NoError(t, err)
		assert.NotEmpty(t, usermap)
	}
}

func TestFetchPullRequests(t *testing.T) {
	if skipTests(t) {
		return
	}
	a := NewTFSAPI(context.Background(), hclog.NewNullLogger(), "1234567890", "tfs", &conf)
	repos, _, err := a.FetchRepos([]string{}, []string{})
	assert.NoError(t, err)
	for _, repo := range repos {
		_, _, err := a.FetchPullRequests(repo.RefID)
		assert.NoError(t, err)
	}
}
func TestFetchPullRequestComments(t *testing.T) {
	if skipTests(t) {
		return
	}
	a := NewTFSAPI(context.Background(), hclog.NewNullLogger(), "1234567890", "tfs", &conf)
	repos, _, err := a.FetchRepos([]string{}, []string{})
	assert.NoError(t, err)
	for _, repo := range repos {
		prs, _, err := a.FetchPullRequests(repo.RefID)
		assert.NoError(t, err)
		for _, p := range prs {
			_, err := a.FetchPullRequestComments(repo.RefID, p.RefID)
			assert.NoError(t, err)
		}
	}

}
