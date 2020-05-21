package api

import (
	"github.com/pinpt/agent/integrations/jira/commonapi"
)

func FieldsAll(qc QueryContext) (res []commonapi.CustomField, rerr error) {

	objectPath := "field"

	qc.Logger.Debug("fields request")

	var rr []struct {
		ID   string `json:"id"`
		Key  string `json:"key"`
		Name string `json:"name"`
	}

	err := qc.Req.Get(objectPath, nil, &rr)
	if err != nil {
		rerr = err
		return
	}

	for _, data := range rr {
		item := commonapi.CustomField{}
		item.ID = data.Key
		item.Name = data.Name
		res = append(res, item)
	}

	return
}
