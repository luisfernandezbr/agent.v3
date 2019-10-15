package structmarshal

import "encoding/json"

func MapToStruct(m map[string]interface{}, target interface{}) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}

func StructToMap(s interface{}) (map[string]interface{}, error) {
	res := map[string]interface{}{}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func AnyToAny(in interface{}, out interface{}) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, out)
	return err
}

func StructToStruct(in interface{}, out interface{}) error {
	return AnyToAny(in, out)
}
