package api

import (
	"net/url"
)

// Groups fetch groups
func Groups(qc QueryContext) (groupNames []string, err error) {
	qc.Logger.Debug("groups request")

	objectPath := "groups"
	params := url.Values{}
	params.Set("per_page", "100")

	var groups []struct {
		Name string `json:"name,omitempty"`
	}

	_, err = qc.Request(objectPath, params, &groups)
	if err != nil {
		return
	}

	for _, group := range groups {
		groupNames = append(groupNames, group.Name)
	}

	return
}
