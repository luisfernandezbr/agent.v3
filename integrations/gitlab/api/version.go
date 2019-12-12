package api

func ApiVersion(qc QueryContext) (apiVersion string, err error) {
	qc.Logger.Debug("groups request")

	objectPath := "version"

	var vInfo struct {
		Version string `json:"version"`
	}

	_, err = qc.Request(objectPath, nil, &vInfo)
	if err != nil {
		return
	}

	apiVersion = vInfo.Version

	return
}
