package api

func ServerVersion(qc QueryContext) (serverVersion string, err error) {
	qc.Logger.Debug("groups request")

	objectPath := "version"

	var vInfo struct {
		Version string `json:"version"`
	}

	_, err = qc.Request(objectPath, nil, &vInfo)
	if err != nil {
		return
	}

	serverVersion = vInfo.Version

	return
}
