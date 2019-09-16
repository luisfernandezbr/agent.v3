package api

import "net/url"

func Teams(qc QueryContext) (groupNames []string, err error) {
	qc.Logger.Debug("groups request")

	objectPath := "teams"
	params := url.Values{}
	params.Set("pagelen", "100")
	params.Set("role", "member")

	var groups []struct {
		Name string `json:"username"`
	}

	_, err = qc.Request(objectPath, params, true, &groups)
	if err != nil {
		return
	}

	for _, group := range groups {
		groupNames = append(groupNames, group.Name)
	}

	return
}
