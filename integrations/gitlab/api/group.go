package api

import (
	"net/url"
	"strings"

	"github.com/hashicorp/go-hclog"
)

// GroupsAll all groups
func GroupsAll(qc QueryContext) (groupNames []string, err error) {
	err = PaginateStartAt(qc.Logger, func(log hclog.Logger, paginationParams url.Values) (page PageInfo, _ error) {
		pi, groups, err := groups(qc, paginationParams)
		if err != nil {
			return pi, err
		}
		for _, groupName := range groups {
			groupNames = append(groupNames, groupName)
		}
		return pi, nil
	})
	return
}

// Groups fetch groups
func groups(qc QueryContext, params url.Values) (pi PageInfo, groupNames []string, err error) {
	qc.Logger.Debug("groups request")

	objectPath := "groups"
	params.Set("per_page", "100")

	var groups []struct {
		FullPath string `json:"full_path"`
	}

	pi, err = qc.Request(objectPath, params, &groups)
	if err != nil {
		return
	}

	for _, group := range groups {
		groupNames = append(groupNames, strings.ToLower(group.FullPath))
	}

	return
}
