package api

func PREditTitle(qc QueryContext, id, title string) error {
	qc.Logger.Info("editing pr title", "pr", id, "title", title)

	query := `
	mutation($id:ID! $title:String!) {
		updatePullRequest(input:{
			pullRequestId: $id,
			title: $title
		}) {
			clientMutationId
		}
	}
	`
	vars := map[string]interface{}{
		"id":    id,
		"title": title,
	}
	var res interface{}
	return qc.Request(query, vars, &res)
}
