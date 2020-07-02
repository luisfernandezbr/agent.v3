package api

import (
	"net/url"
	"strconv"

	"github.com/hashicorp/go-hclog"
)

type Group struct {
	ID       string
	FullPath string
}

// GroupsAll all groups
func GroupsAll(qc QueryContext) (allGroups []*Group, err error) {
	err = PaginateStartAt(qc.Logger, func(log hclog.Logger, paginationParams url.Values) (page PageInfo, _ error) {
		pi, groups, err := groups(qc, paginationParams)
		if err != nil {
			return pi, err
		}
		allGroups = append(allGroups, groups...)
		return pi, nil
	})
	return
}

// Groups fetch groups
func groups(qc QueryContext, params url.Values) (pi PageInfo, groups []*Group, err error) {

	params.Set("per_page", "100")

	qc.Logger.Debug("groups request", "params", params)

	objectPath := "groups"

	var rgroups []struct {
		ID       int64  `json:"id"`
		FullPath string `json:"full_path"`
	}

	pi, err = qc.Request(objectPath, params, &rgroups)
	if err != nil {
		return
	}

	for _, group := range rgroups {
		groups = append(groups, &Group{
			ID:       strconv.FormatInt(group.ID, 10),
			FullPath: group.FullPath,
		})
	}

	return
}
