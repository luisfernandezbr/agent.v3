package jiracommonapi

func ApiVersion(qc QueryContext) (apiVersion string, err error) {

	objectPath := "serverInfo"

	var serverInfo struct {
		Version string `json:"version"`
	}

	err = qc.Request(objectPath, nil, &serverInfo)
	if err != nil {
		return
	}

	apiVersion = serverInfo.Version

	return
}
