package api

import (
	"strings"
	"time"

	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/integration-sdk/codequality"
)

type metricsResponse struct {
	Measures []*struct {
		Metric  string `json:"metric"`
		History []*struct {
			Date  string `json:"date"`
			Value string `json:"value"`
		} `json:"history"`
	} `json:"measures"`
}

// FetchMetrics _
func (a *SonarqubeAPI) FetchMetrics(project *codequality.Project, fromDate time.Time) ([]*codequality.Metric, error) {
	project.ToMap() // need to call setDefaults so that ID is set
	metricKeys := strings.Join(a.metrics, ",")
	ur := "/measures/search_history?p=1&ps=500&component=" + project.Identifier + "&metrics=" + metricKeys
	val := []metricsResponse{}
	err := a.doRequest("GET", ur, fromDate, &val)
	if err != nil {
		return nil, err
	}

	var res []*codequality.Metric
	for _, each := range val {
		for _, measure := range each.Measures {
			for _, metric := range measure.History {
				if metric.Value != "" {
					created, err := time.Parse("2006-01-02T15:04:05-0700", metric.Date)
					if err != nil {
						return nil, err
					}
					metr := &codequality.Metric{
						Name:      measure.Metric,
						Value:     metric.Value,
						RefID:     hash.Values(project.ID, metric.Date, measure.Metric),
						RefType:   "sonarqube",
						ProjectID: project.ID,
					}
					date.ConvertToModel(created, &metr.CreatedDate)
					res = append(res, metr)
				}
			}
		}
	}
	return res, nil
}
