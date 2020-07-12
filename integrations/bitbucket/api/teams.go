package api

import "net/url"

func Teams(qc QueryContext) (teamNames []string, err error) {
	qc.Logger.Debug("teams request")

	objectPath := "teams"
	params := url.Values{}
	params.Set("pagelen", "100")
	params.Set("role", "member")

	var teams []struct {
		Name string `json:"username"`
	}

	_, err = qc.Request(objectPath, params, true, &teams, "")
	if err != nil {
		return
	}

	for _, obj := range teams {
		teamNames = append(teamNames, obj.Name)
	}

	if len(teamNames) == 100 {
		qc.Logger.Error("bitbucket integration supports 100 teams at most")
	}

	return
}
