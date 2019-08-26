package api

import (
	"context"
	"testing"
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/stretchr/testify/assert"
)

var url = ""
var authToken = ""

var metricsArray = []string{
	"complexity", "code_smells", "new_code_smells", "sqale_rating", "reliability_rating", "security_rating", "coverage", "new_coverage", "test_success_density", "new_technical_debt",
}

func TestFetchProjects(t *testing.T) {
	sonarapi := NewSonarqubeAPI(context.Background(), url, authToken, metricsArray)
	projects, err := sonarapi.FetchProjects(time.Time{})
	assert.NoError(t, err)
	assert.NotEmpty(t, projects)
}
func TestValidate(t *testing.T) {
	sonarapi := NewSonarqubeAPI(context.Background(), url, authToken, metricsArray)
	valid, err := sonarapi.Validate()
	assert.NoError(t, err)
	assert.True(t, valid)
}

func TestFetchMetrics(t *testing.T) {
	sonarapi := NewSonarqubeAPI(context.Background(), url, authToken, metricsArray)
	measures, err := sonarapi.FetchMetrics("key-1", time.Time{})
	assert.NoError(t, err)
	assert.NotEmpty(t, measures)
}

type testDate struct {
	// Epoch the date in epoch format
	Epoch int64 `json:"epoch" bson:"epoch" yaml:"epoch" faker:"-"`
	// Offset the timezone offset from GMT
	Offset int64 `json:"offset" bson:"offset" yaml:"offset" faker:"-"`
	// Rfc3339 the date in RFC3339 format
	Rfc3339 string `json:"rfc3339" bson:"rfc3339" yaml:"rfc3339" faker:"-"`
}

func TestFetchAllMetrics(t *testing.T) {
	sonarapi := NewSonarqubeAPI(context.Background(), url, authToken, metricsArray)
	projects, err := sonarapi.FetchProjects(time.Time{})
	assert.NoError(t, err)
	assert.NotEmpty(t, projects)
	metrics, err := sonarapi.FetchAllMetrics(projects, time.Time{})
	assert.NoError(t, err)
	assert.NotEmpty(t, metrics)
	for _, m := range metrics {
		var dt testDate
		date.ConvertToModel(m.Date, &dt)
		assert.NotEmpty(t, dt)
	}
}
