package api

func ReposAllIDs(qc QueryContext, idChan chan []string) error {
	defer close(idChan)

	return PaginateRegular(func(query string) (pi PageInfo, _ error) {
		pi, ids, err := ReposPageIDs(qc, query)
		if err != nil {
			return pi, err
		}
		idChan <- ids
		return pi, nil
	})
}

func ReposPageIDs(qc QueryContext, queryParams string) (pi PageInfo, ids IDs, _ error) {

	query := `
	query {
		viewer {
			organization(login:"pinpt"){
				repositories(` + queryParams + `) {
					totalCount
					pageInfo {
						hasNextPage
						endCursor
						hasPreviousPage
						startCursor
					}
					nodes {
						id
					}
				}
			}
		}
	}
	`

	var res struct {
		Data struct {
			Viewer struct {
				Organization struct {
					Repositories struct {
						TotalCount int      `json:"totalCount"`
						PageInfo   PageInfo `json:"pageInfo"`
						Nodes      []struct {
							ID string `json:"id"`
						} `json:"nodes"`
					} `json:"repositories"`
				} `json:"organization"`
			} `json:"viewer"`
		} `json:"data"`
	}

	err := qc.Request(query, &res)
	if err != nil {
		return pi, ids, err
	}

	repositories := res.Data.Viewer.Organization.Repositories
	repoNodes := repositories.Nodes

	if len(repoNodes) == 0 {
		qc.Logger.Warn("no repos found")
		return pi, ids, nil
	}

	var batch IDs
	for _, data := range repoNodes {
		batch = append(batch, data.ID)
	}

	return repositories.PageInfo, batch, nil
}
