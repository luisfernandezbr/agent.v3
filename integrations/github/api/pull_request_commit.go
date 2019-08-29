package api

func PullRequestCommitsPage(
	qc QueryContext,
	pullRequestRefID string,
	queryParams string) (pi PageInfo, res []string, _ error) {

	if pullRequestRefID == "" {
		panic("missing pr id")
	}

	qc.Logger.Debug("pull_request_commits request", "pr", pullRequestRefID, "q", queryParams)

	query := `
	query {
		node (id: "` + pullRequestRefID + `") {
			... on PullRequest {
				commits(` + queryParams + `) {
					totalCount
					pageInfo {
						hasNextPage
						endCursor
						hasPreviousPage
						startCursor
					}
					nodes {
						commit {
							oid
						}
					}
				}
			}
		}
	}
	`

	var requestRes struct {
		Data struct {
			Node struct {
				Commits struct {
					TotalCount int      `json:"totalCount"`
					PageInfo   PageInfo `json:"pageInfo"`
					Nodes      []struct {
						Commit struct {
							OID string `json:"oid"`
						} `json:"commit"`
					} `json:"nodes"`
				} `json:"commits"`
			} `json:"node"`
		} `json:"data"`
	}

	err := qc.Request(query, &requestRes)
	if err != nil {
		return pi, res, err
	}

	nodesContainer := requestRes.Data.Node.Commits
	nodes := nodesContainer.Nodes
	//qc.Logger.Info("got comments", "n", len(nodes))
	for _, data := range nodes {
		res = append(res, data.Commit.OID)
	}

	return nodesContainer.PageInfo, res, nil
}
