## API used for the work integration

### FetchChangelogs
#### fetchItemIDs
For every project:

API:   `{project_id}/_apis/wit/wiql`
Query: `Select System.ID From WorkItems` if incremental ` WHERE System.ChangedDate > {date}`
```
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
```
#### fetchChangeLog
For every project and item:

`{project_id}/_apis/wit/workItems/{item_id}/updates`
```
type changelogResponse struct {
	Fields map[string]struct {
		NewValue interface{} `json:"newValue"`
		OldValue interface{} `json:"oldvalue"`
	} `json:"fields"`
	ID          int64     `json:"id"`
	RevisedDate time.Time `json:"revisedDate"`
	URL         string    `json:"url"`
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
	RevisedBy struct {
		Descriptor  string `json:"descriptor"`
		DisplayName string `json:"displayName"`
		ID          string `json:"id"`
		ImageURL    string `json:"imageUrl"`
		UniqueName  string `json:"uniqueName"`
		URL         string `json:"url"`
	} `json:"revisedBy"`
}
```
### FetchWorkItems
For every project pass in all the item ids:

`{project_id}/_apis/wit/workitems?ids={item_ids}`
```
type WorkItemResponse struct {
	Fields struct {
		AssignedTo     struct {
			Descriptor  string `json:"descriptor"`
			DisplayName string `json:"displayName"`
			ID          string `json:"id"`
			ImageURL    string `json:"imageUrl"`
			UniqueName  string `json:"uniqueName"`
			URL         string `json:"url"`
		} `json:"System.AssignedTo"`
		CreatedDate    time.Time     `json:"System.CreatedDate"`
		CreatedBy      struct {
			Descriptor  string `json:"descriptor"`
			DisplayName string `json:"displayName"`
			ID          string `json:"id"`
			ImageURL    string `json:"imageUrl"`
			UniqueName  string `json:"uniqueName"`
			URL         string `json:"url"`
		} `json:"System.CreatedBy"`
		DueDate        time.Time     `json:"Microsoft.VSTS.Scheduling.DueDate"` // ??
		TeamProject    string        `json:"System.TeamProject"`
		Priority       int           `json:"Microsoft.VSTS.Common.Priority"`
		ResolvedReason string        `json:"Microsoft.VSTS.Common.ResolvedReason"`
		ResolvedDate   time.Time     `json:"Microsoft.VSTS.Common.ResolvedDate"`
		State          string        `json:"System.State"`
		Tags           string        `json:"System.Tags"`
		Title          string        `json:"System.Title"`
		WorkItemType   string        `json:"System.WorkItemType"`
		ChangedDate    time.Time     `json:"System.ChangedDate"`
	} `json:"fields"`
	ID  int    `json:"id"`
	URL string `json:"url"`
}
```
### FetchProjects
`_apis/projects/`
```
type projectResponse struct {
	projectResponseLight
	Revision    int64  `json:"revision"`
	State       string `json:"state"`
	URL         string `json:"url"`
	Visibility  string `json:"visibility"`
	Description string `json:"description"`
}
```
### FetchSprints
#### fetchTeams
For every project:

`_apis/projects/{project_id}/teams`
```
type teamsResponse struct {
	Description string `json:"description"`
	ID          string `json:"id"`
	IdentityURL string `json:"identityUrl"`
	Name        string `json:"name"`
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	URL         string `json:"url"`
}
```
#### fetchSprint
For every project and team:

`{project_id}/{team_id}/_apis/work/teamsettings/iterations`
```
type WorkItemResponse struct {
	Fields struct {
		AssignedTo     struct {
			Descriptor  string `json:"descriptor"`
			DisplayName string `json:"displayName"`
			ID          string `json:"id"`
			ImageURL    string `json:"imageUrl"`
			UniqueName  string `json:"uniqueName"`
			URL         string `json:"url"`
		} `json:"System.AssignedTo"`
		CreatedDate    time.Time     `json:"System.CreatedDate"`
		CreatedBy      struct {
			Descriptor  string `json:"descriptor"`
			DisplayName string `json:"displayName"`
			ID          string `json:"id"`
			ImageURL    string `json:"imageUrl"`
			UniqueName  string `json:"uniqueName"`
			URL         string `json:"url"`
		} `json:"System.CreatedBy"`
		DueDate        time.Time     `json:"Microsoft.VSTS.Scheduling.DueDate"` // ??
		TeamProject    string        `json:"System.TeamProject"`
		Priority       int           `json:"Microsoft.VSTS.Common.Priority"`
		ResolvedReason string        `json:"Microsoft.VSTS.Common.ResolvedReason"`
		ResolvedDate   time.Time     `json:"Microsoft.VSTS.Common.ResolvedDate"`
		State          string        `json:"System.State"`
		Tags           string        `json:"System.Tags"`
		Title          string        `json:"System.Title"`
		WorkItemType   string        `json:"System.WorkItemType"`
		ChangedDate    time.Time     `json:"System.ChangedDate"`
	} `json:"fields"`
	ID  int    `json:"id"`
	URL string `json:"url"`
}
```
### FetchWorkUsers
For every project and team:

`_apis/projects/{project_id}/teams/{team_id}/members`
```
type usersResponse struct {
	Descriptor  string `json:"descriptor"`
	DisplayName string `json:"displayName"`
	ID          string `json:"id"`
	ImageURL    string `json:"imageUrl"`
	UniqueName  string `json:"uniqueName"`
	URL         string `json:"url"`
}
```