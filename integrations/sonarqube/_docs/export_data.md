## API used in Sonarqube

### FetchProjects
`/components/search?p=1&ps=500&qualifiers=TRK`
```
type projectsResponse struct {
	Components []*struct {
		ID           string `json:"id"`
		Key          string `json:"key"`
		Name         string `json:"name"`
		Organization string `json:"organization"`
		Qualifier    string `json:"qualifier"`
		Project      string `json:"project"`
	} `json:"components"`
}
```
### FetchMetrics
For every project send all keys:

`/measures/search_history?p=1&ps=500&component={project_id}&metrics={metric_keys}`
```
type metricsResponse struct {
	Measures []*struct {
		Metric  string `json:"metric"`
		History []*struct {
			Date  string `json:"date"`
			Value string `json:"value"`
		} `json:"history"`
	} `json:"measures"`
}
```