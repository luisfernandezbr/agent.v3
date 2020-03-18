package api

import (
	"net/http"

	"github.com/pinpt/agent/pkg/requests"
	pstrings "github.com/pinpt/go-common/strings"
)

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

func OrgsEnterpriseAll(qc QueryContext) (res []Org, rerr error) {
	err := PaginateV3(func(u string) (responseHeaders http.Header, rerr error) {
		subRes, h, err := OrgsEnterprisePage(qc, u)
		if err != nil {
			rerr = err
			return
		}
		for _, obj := range subRes {
			res = append(res, obj)
		}
		return h, nil
	})
	if err != nil {
		return nil, err
	}
	return
}

func OrgsEnterprisePage(qc QueryContext, u string) (res []Org, header http.Header, rerr error) {
	if u == "" {
		u = pstrings.JoinURL(qc.APIURL3, "organizations")
	}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		rerr = err
		return
	}

	// This is not necessary in certain server configurations.
	req.Header.Set("Authorization", "token "+qc.AuthToken)

	var respJSON []struct {
		Login string `json:"login"`
	}

	reqs := requests.New(qc.Logger, qc.Clients.TLSInsecure)

	resp, err := reqs.JSON(req, &respJSON)
	if err != nil {
		rerr = err
		return
	}

	for _, data := range respJSON {
		item := Org{}
		item.Login = data.Login
		res = append(res, item)
	}

	return res, resp.Header, nil
}
