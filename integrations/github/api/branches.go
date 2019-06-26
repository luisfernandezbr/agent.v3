package api

import (
	pjson "github.com/pinpt/go-common/json"
)

func BranchNames(qc QueryContext, repoID string, res chan []string) error {
	return PaginateRegular(func(query string) (pi PageInfo, _ error) {
		pi, names, err := BranchNamesPage(qc, repoID, query)
		if err != nil {
			return pi, err
		}
		res <- names
		return pi, nil
	})
}

func BranchNamesPage(qc QueryContext, repoID, queryParams string) (pi PageInfo, res []string, _ error) {

	qc.Logger.Debug("branch names request", "repo", repoID, "q", queryParams)

	query := `
	query {
		node (id: ` + pjson.Stringify(repoID) + `) {
			... on Repository {
				refs(refPrefix:"refs/heads/" ` + queryParams + `){
					pageInfo {
						hasNextPage
						endCursor
						hasPreviousPage
						startCursor
					}
					edges {
						node {
							name
						}
					}
				}
			}
		}
	}
	`

	var reqRes struct {
		Data struct {
			Node struct {
				Refs struct {
					PageInfo PageInfo `json:"pageInfo"`
					Edges    []struct {
						Node struct {
							Name string `json:"name"`
						} `json:"node"`
					} `json:"edges"`
				} `json:"refs"`
			} `json:"node"`
		} `json:"data"`
	}

	err := qc.Request(query, &reqRes)
	if err != nil {
		return pi, res, err
	}

	edges := reqRes.Data.Node.Refs.Edges

	if len(edges) == 0 {
		qc.Logger.Warn("no branches found")
		return pi, res, nil
	}

	for _, data := range edges {
		res = append(res, data.Node.Name)
	}

	return reqRes.Data.Node.Refs.PageInfo, res, nil
}
