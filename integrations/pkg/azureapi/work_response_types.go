package azureapi

import "time"

type changelogField struct {
	NewValue interface{} `json:"newValue"`
	OldValue interface{} `json:"oldvalue"`
}
type changelogResponse struct {
	Fields      map[string]changelogField `json:"fields"`
	ID          int64                     `json:"id"`
	RevisedDate time.Time                 `json:"revisedDate"`
	URL         string                    `json:"url"`
	Relations   struct {
		Added []struct {
			Attributes struct {
				Name string `json:"name"`
			} `json:"attributes"`
			URL string `json:"url"`
		} `json:"added"`
		Removed []struct {
			Attributes struct {
				Name string `json:"name"`
			} `json:"attributes"`
			URL string `json:"url"`
		} `json:"removed"`
	} `json:"relations"`
	RevisedBy usersResponse `json:"revisedBy"`
}

type workItemOperation struct {
	Op    string  `json:"op"`
	Path  string  `json:"path"`
	From  *string `json:"from"`
	Value string  `json:"value"`
}

type workItemsResponse struct {
	AsOf    time.Time `json:"asOf"`
	Columns []struct {
		Name          string `json:"name"`
		ReferenceName string `json:"referenceName"`
		URL           string `json:"url"`
	} `json:"columns"`
	QueryResultType string `json:"queryResultType"`
	QueryType       string `json:"queryType"`
	SortColumns     []struct {
		Descending bool `json:"descending"`
		Field      struct {
			Name          string `json:"name"`
			ReferenceName string `json:"referenceName"`
			URL           string `json:"url"`
		} `json:"field"`
	} `json:"sortColumns"`
	WorkItems []struct {
		ID  int64  `json:"id"`
		URL string `json:"url"`
	} `json:"workItems"`
}

type sprintsResponse struct {
	Attributes struct {
		FinishDate time.Time `json:"finishDate"`
		StartDate  time.Time `json:"startDate"`
		TimeFrame  string    `json:"timeFrame"` // past, current, future
	} `json:"attributes"`
	ID   string `json"id"`
	Name string `json"name"`
	Path string `json"path"`
	URL  string `json"url"`
}
