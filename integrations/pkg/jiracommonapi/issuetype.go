package jiracommonapi

func IssueTypes(qc QueryContext) (res []string, rerr error) {

	objectPath := "issuetype"

	qc.Logger.Debug("fields request")

	var issueTypes []struct {
		Name string `json:"name"`
	}

	err := qc.Request(objectPath, nil, &issueTypes)
	if err != nil {
		rerr = err
		return
	}

	for _, issueType := range issueTypes {
		res = append(res, issueType.Name)
	}

	return
}
