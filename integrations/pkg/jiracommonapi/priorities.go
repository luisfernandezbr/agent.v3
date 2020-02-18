package jiracommonapi

func Priorities(qc QueryContext) (res []string, rerr error) {

	objectPath := "priority"

	var rawPriorities []struct {
		Name string `json:"name"`
	}

	err := qc.Req.Get(objectPath, nil, &rawPriorities)
	if err != nil {
		rerr = err
		return
	}

	for _, priority := range rawPriorities {
		res = append(res, priority.Name)
	}

	return
}
