package jiracommonapi

func IssueTypes(qc QueryContext) (res []string, rerr error) {

	objectPath := "issuetype"

	var issueTypes []struct {
		Name string `json:"name"`
	}

	err := qc.Request(objectPath, nil, &issueTypes)
	if err != nil {
		rerr = err
		return
	}

	m := make(map[string]bool)

	for _, typ := range issueTypes {
		m[typ.Name] = true
	}

	for k := range m {
		res = append(res, k)
	}

	return
}
