package jiracommonapi

func Status(qc QueryContext) (res []string, rerr error) {

	objectPath := "status"

	var rawStatuses []struct {
		Name string `json:"name"`
	}

	err := qc.Request(objectPath, nil, &rawStatuses)
	if err != nil {
		rerr = err
		return
	}

	m := make(map[string]bool)

	for _, status := range rawStatuses {
		m[status.Name] = true
	}

	for k := range m {
		res = append(res, k)
	}

	return
}
