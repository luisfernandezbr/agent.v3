package jiracommonapi

func Priorities(qc QueryContext) (res []string, rerr error) {

	objectPath := "priority"

	qc.Logger.Debug("fields request")

	var rawPriorities []struct {
		Name string `json:"name"`
	}

	err := qc.Request(objectPath, nil, &rawPriorities)
	if err != nil {
		rerr = err
		return
	}

	for _, priority := range rawPriorities {
		res = append(res, priority.Name)
	}

	return
}
