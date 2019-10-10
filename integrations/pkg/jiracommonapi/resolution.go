package jiracommonapi

func Resolution(qc QueryContext) (res []string, rerr error) {

	objectPath := "resolution"

	var resolutions []struct {
		Name string `json:"name"`
	}

	err := qc.Request(objectPath, nil, &resolutions)
	if err != nil {
		rerr = err
		return
	}

	for _, resolution := range resolutions {
		res = append(res, resolution.Name)
	}

	return
}
