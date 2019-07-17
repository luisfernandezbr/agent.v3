package structmarshal

import "encoding/json"

func MapToStruct(m map[string]interface{}, target interface{}) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &target)
}
