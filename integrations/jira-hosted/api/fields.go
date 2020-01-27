package api

import (
	"github.com/pinpt/integration-sdk/work"
)

func FieldsAll(qc QueryContext) (res []*work.CustomField, rerr error) {

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
		item := &work.CustomField{}
		item.CustomerID = qc.CustomerID
		item.RefType = "jira"
		item.RefID = data.ID
		item.Key = data.ID
		item.Name = data.Name
		res = append(res, item)
	}

	return
}
