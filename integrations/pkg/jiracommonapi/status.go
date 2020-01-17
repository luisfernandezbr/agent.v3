package jiracommonapi

func Status(qc QueryContext) (res []string, rerr error) {

	objectPath := "status"

	var rawStatuses []struct {
		Name string `json:"name"`
	}

	err := qc.Request(objectPath, nil, &rawStatuses)
	if err != nil {
		rerr = err
		return
	}

	m := make(map[string]bool)

	for _, status := range rawStatuses {
		m[status.Name] = true
	}

	for k := range m {
		res = append(res, k)
	}

	return
}

type StatusDetail struct {
	Name           string `json:"name"`
	StatusCategory struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"statusCategory"`
}

func StatusWithDetail(qc QueryContext) ([]StatusDetail, []string, error) {
	objectPath := "status"

	var detail []StatusDetail
	m := make(map[string]bool)

	err := qc.Request(objectPath, nil, &detail)
	if err != nil {
		return nil, nil, err
	}

	for _, status := range detail {
		m[status.Name] = true
	}

	var res []string

	for k := range m {
		res = append(res, k)
	}

	return detail, res, nil
}
