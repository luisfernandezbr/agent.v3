package api

import "errors"

// Org contains the data needed for exporting other resources depending on it
type Org struct {
	Login string
}

func OrgsAll(qc QueryContext) (res []Org, rerr error) {

	query := `
	query {
		viewer {
			organizations(first:100){
				totalCount
				nodes {
					login
				}
			}
		}
	}
	`

	var resp struct {
		Data struct {
			Viewer struct {
				Organizations struct {
					TotalCount int `json:"totalCount"`
					Nodes      []struct {
						Login string `json:"login"`
					} `json:"nodes"`
				} `json:"organizations"`
			} `json:"viewer"`
		} `json:"data"`
	}

	err := qc.Request(query, &resp)
	if err != nil {
		rerr = err
		return
	}

	orgs := resp.Data.Viewer.Organizations
	orgNodes := orgs.Nodes

	if len(orgNodes) == 0 {
		rerr = errors.New("no github organizations found in this account")
		return
	}

	if orgs.TotalCount == 0 {
		panic("missing field totalCount")
	}

	if orgs.TotalCount == 100 {
		panic("this account has 100 or more organizations, only <100 supported at this time")
	}

	for _, data := range orgNodes {
		item := Org{}
		item.Login = data.Login
		res = append(res, item)
	}

	return
}
