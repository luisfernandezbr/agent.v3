package jiracommonapi

func Labels(qc QueryContext) (res []string, rerr error) {

	objectPath := "label"

	var labelsInfo struct {
		Values []string `json:"values"`
	}

	err := qc.Request(objectPath, nil, &labelsInfo)
	if err != nil {
		rerr = err
		return
	}

	for _, label := range labelsInfo.Values {
		res = append(res, label)
	}

	return
}
