package api

import (
	"time"
)

func (a *SonarqubeAPI) APIVersion() (apiVersion string, err error) {

	var serverInfo struct {
		Version string `json:"version"`
	}

	err = a.doRequest("GET", "/server", time.Time{}, &serverInfo)
	if err != nil {
		return
	}

	apiVersion = serverInfo.Version

	return
}
