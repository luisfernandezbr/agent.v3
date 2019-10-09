package api

import (
	"github.com/pinpt/agent.next/integrations/pkg/jiracommonapi"
	"github.com/pinpt/integration-sdk/agent"
)

func UsersAllx(qc QueryContext) (res []*agent.UserResponseUsers, rerr error) {

	objectPath := "user/search?query="

	qc.Logger.Debug("users request")

	var rr []jiracommonapi.User

	err := qc.Request(objectPath, nil, &rr)
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
