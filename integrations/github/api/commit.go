package api

import (
	pjson "github.com/pinpt/go-common/json"
)

type CommitAuthor struct {
	CommitHash string
	//BranchName    string
	AuthorRefID    string
	CommitterRefID string
}

func CommitsPage(
	qc QueryContext,
	repoRefID string, branchName string,
	queryParams string) (pi PageInfo, res []CommitAuthor, _ error) {

	qc.Logger.Debug("commits request", "repo", repoRefID, "branchName", branchName, "q", queryParams)

	query := `
	query {
		node (id: "` + repoRefID + `") {
			... on Repository {
				ref(qualifiedName: ` + pjson.Stringify(branchName) + `){
					target {
						... on Commit {
							history(` + queryParams + `){
								totalCount
								pageInfo {
									hasNextPage
									endCursor
									hasPreviousPage
									startCursor
								}
								nodes {
									oid
									author {
										user {
											login
										}
									}
									committer {
										user {
											login
										}
									}
								}	
							}
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
				Ref struct {
					Target struct {
						History struct {
							TotalCount int      `json:"totalCount"`
							PageInfo   PageInfo `json:"pageInfo"`
							Nodes      []struct {
								OID    string `json:"oid"`
								Author struct {
									User struct {
										Login string `json:"login"`
									} `json:"user"`
								} `json:"author"`
								Committer struct {
									User struct {
										Login string `json:"login"`
									} `json:"user"`
								} `json:"committer"`
							} `json:"nodes"`
						} `json:"history"`
					} `json:"target"`
				} `json:"ref"`
			} `json:"node"`
		} `json:"data"`
	}

	err := qc.Request(query, &requestRes)
	if err != nil {
		return pi, res, err
	}

	//qc.Logger.Info(fmt.Sprintf("object %+v", requestRes))

	commits := requestRes.Data.Node.Ref.Target.History
	commitNodes := commits.Nodes

	for _, data := range commitNodes {
		item := CommitAuthor{}
		item.CommitHash = data.OID
		item.AuthorRefID, err = qc.UserLoginToRefID(data.Author.User.Login)
		if err != nil {
			panic(err)
		}
		item.CommitterRefID, err = qc.UserLoginToRefID(data.Committer.User.Login)
		if err != nil {
			panic(err)
		}
		res = append(res, item)
	}

	return commits.PageInfo, res, nil
}
