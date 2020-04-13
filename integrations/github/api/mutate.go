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

func PREditBody(qc QueryContext, id, body string) error {
	qc.Logger.Info("editing pr body", "pr", id, "body", body)

	query := `
	mutation($id:ID! $body:String!) {
		updatePullRequest(input:{
			pullRequestId: $id,
			body: $body
		}) {
			clientMutationId
		}
	}
	`
	vars := map[string]interface{}{
		"id":   id,
		"body": body,
	}
	var res interface{}
	return qc.Request(query, vars, &res)
}
