package api

import (
	"net/url"

	"github.com/pinpt/agent.next/integrations/pkg/jiracommonapi"
	"github.com/pinpt/integration-sdk/agent"
)

func UsersOnboard(qc QueryContext) (res []*agent.UserResponseUsers, rerr error) {

	q := url.Values{}
	q.Set("maxResults", "1000")
	// Previous version of the agent iterated from a-z to fetch all users, but . seems to work fine for me
	q.Set("username", ".")
	objectPath := "user/search"

	qc.Logger.Debug("users onboard")

	var rr []jiracommonapi.User

	err := qc.Request(objectPath, q, &rr)
	if err != nil {
		rerr = err
		return
	}

	for _, data := range rr {
		item := &agent.UserResponseUsers{}
		item.CustomerID = qc.CustomerID
		item.Active = data.Active
		item.RefType = "jira"
		item.Username = data.Name
		item.Name = data.DisplayName
		item.Emails = []string{data.EmailAddress}
		item.AvatarURL = &data.Avatars.Large
		for _, g := range data.Groups.Groups {
			g2 := agent.UserResponseUsersGroups{}
			g2.GroupID = g.Name
			g2.Name = g.Name
			item.Groups = append(item.Groups, g2)
		}
		res = append(res, item)
	}

	return
}
