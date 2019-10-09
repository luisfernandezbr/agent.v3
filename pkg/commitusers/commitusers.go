package commitusers

import (
	"encoding/json"
	"fmt"
)

const TableName = "sourcecode.CommitUser"

type CommitUser struct {
	CustomerID string
	Email      string
	Name       string
	SourceID   string
}

func (s CommitUser) Validate() error {
	if s.CustomerID == "" || s.Email == "" || s.Name == "" {
		return fmt.Errorf("missing required field for user: %+v", s)
	}
	return nil
}

func (s CommitUser) Stringify() string {
	b, _ := json.Marshal(s.ToMap())
	return string(b)
}

func (s CommitUser) ToMap() map[string]interface{} {
	res := map[string]interface{}{}
	res["customer_id"] = s.CustomerID
	res["email"] = s.Email
	res["name"] = s.Name
	res["source_id"] = s.SourceID
	return res
}
