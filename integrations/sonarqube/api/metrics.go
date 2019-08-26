package api

import (
	"strings"
	"time"

	"github.com/pinpt/go-common/hash"
)

// FetchMetrics _
func (a *SonarqubeAPI) FetchMetrics(componentID string, fromDate time.Time) ([]*Measure, error) {

	type metricsResponse struct {
		Measures []*Measure `json:"measures"`
	}

	metricKeys := strings.Join(a.metrics, ",")
	ur := "/measures/search_history?p=1&ps=500&component=" + componentID + "&metrics=" + metricKeys
	val := metricsResponse{}
	err := a.doRequest("GET", ur, fromDate, &val)
	if err != nil {
		return nil, err
	}

	// Remove metrics that have empty value
	for _, m := range val.Measures {
		r := []*RawMetric{}
		for i := range m.History {
			h := m.History[i]
			if h.Value != "" {
				r = append(r, h)
			}
		}
		m.History = r
	}

	var measures []*Measure
	for _, w := range val.Measures {
		measures = append(measures, w)
	}
	return measures, nil
}

// FetchAllMetrics _
func (a *SonarqubeAPI) FetchAllMetrics(projects []*Project, fromDate time.Time) ([]*Metric, error) {
	if projects == nil {
		pro, err := a.FetchProjects(time.Time{})
		if err != nil {
			return nil, err
		}
		projects = pro
	}
	all := []*Metric{}
	for _, proj := range projects {
		mt, err := a.FetchMetrics(proj.Key, fromDate)
		if err != nil {
			return nil, err
		}
		for _, m := range mt {
			for _, h := range m.History {

				date, err := time.Parse("2006-01-02T15:04:05-0700", h.Date)
				if err != nil {
					return nil, err
				}

				all = append(all, &Metric{
					ProjectID:  proj.ID,
					ProjectKey: proj.Key,
					ID:         hash.Values(proj.ID, h.Date, m.Metric),
					Date:       date,
					Metric:     m.Metric,
					Value:      h.Value,
				})
			}
		}
	}
	return all, nil
}
