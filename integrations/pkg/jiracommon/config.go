package jiracommon

type Config struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`

	ExcludedProjects []string `json:"excluded_projects"`
	// Projects specifies a specific projects to process. Ignores excluded_projects in this case. Specify projects using jira key. For example: DE.
	Projects []string `json:"projects"`
}
