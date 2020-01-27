package api

import (
	"github.com/pinpt/agent/integrations/pkg/jiracommonapi"
)

func FieldsAll(qc QueryContext) (res []jiracommonapi.CustomField, rerr error) {

	objectPath := "field"

	qc.Logger.Debug("fields request")

	var rr []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	err := qc.Request(objectPath, nil, &rr)
	if err != nil {
		rerr = err
		return
	}

	for _, data := range rr {
		item := jiracommonapi.CustomField{}
		item.ID = data.ID
		item.Name = data.Name
		res = append(res, item)
	}

	return
}
