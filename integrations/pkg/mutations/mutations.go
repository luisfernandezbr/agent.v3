package mutations

import (
	"fmt"
)

type Model interface {
	ToMap() map[string]interface{}
}

func PluckFields(obj Model, fields ...string) (map[string]interface{}, error) {
	m := obj.ToMap()
	res := map[string]interface{}{}
	copyRequired := func(k string) error {
		if v, ok := m[k].(string); ok {
			if v == "" {
				return fmt.Errorf("%v is an empty string", k)
			}
			res[k] = v
			return nil
		}
		return fmt.Errorf("no %v string on object", k)
	}
	err := copyRequired("id")
	if err != nil {
		return nil, err
	}
	for _, f := range fields {
		res[f] = m[f]
	}
	return res, nil
}
