package api

import (
	"context"
	"os"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/integration-sdk/codequality"
	"github.com/stretchr/testify/assert"
)

var url = ""
var authToken = ""

var metricsArray = []string{
	"complexity", "code_smells", "new_code_smells", "sqale_rating", "reliability_rating", "security_rating", "coverage", "new_coverage", "test_success_density", "new_technical_debt",
}

func skipTests(t *testing.T) bool {
	if os.Getenv("PP_TEST_SONARQUBE_URL") == "" {
		t.Skip("skipping sonarqube tests")
		return true
	}
	return false
}
func TestFetchProjects(t *testing.T) {
	if skipTests(t) {
		return
	}
	sonarapi := NewSonarqubeAPI(context.Background(), hclog.NewNullLogger(), url, authToken, metricsArray)
	projects, err := sonarapi.FetchProjects(time.Time{})
	assert.NoError(t, err)
	assert.NotEmpty(t, projects)
}
func TestValidate(t *testing.T) {
	if skipTests(t) {
		return
	}
	sonarapi := NewSonarqubeAPI(context.Background(), hclog.NewNullLogger(), url, authToken, metricsArray)
	valid, err := sonarapi.Validate()
	assert.NoError(t, err)
	assert.True(t, valid)
}

func TestFetchMetrics(t *testing.T) {
	if skipTests(t) {
		return
	}
	sonarapi := NewSonarqubeAPI(context.Background(), hclog.NewNullLogger(), url, authToken, metricsArray)
	proj := &codequality.Project{
		Identifier: "key-2",
	}
	measures, err := sonarapi.FetchMetrics(proj, time.Time{})
	assert.NoError(t, err)
	assert.NotEmpty(t, measures)
}

func TestFetchAllMetrics(t *testing.T) {
	if skipTests(t) {
		return
	}
	sonarapi := NewSonarqubeAPI(context.Background(), hclog.NewNullLogger(), url, authToken, metricsArray)
	metrics, err := sonarapi.FetchAllMetrics(time.Time{})
	assert.NoError(t, err)
	assert.NotEmpty(t, metrics)
}
