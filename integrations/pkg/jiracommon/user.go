package jiracommon

import (
	"github.com/pinpt/agent.next/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/integration-sdk/work"
)

type Users struct {
	sender     *objsender.NotIncremental
	exported   map[string]bool
	customerID string
}

func NewUsers(customerID string, agent rpcdef.Agent) (*Users, error) {
	s := &Users{}
	s.customerID = customerID
	s.sender = objsender.NewNotIncremental(agent, work.UserTable.String())
	s.exported = map[string]bool{}
	return s, nil
}

func (s *Users) ExportUser(user jiracommonapi.User) error {
	customerID := s.customerID
	pk := user.RefID()
	/*
		TODO: we were hashing user key before, not sure why, needs checking
		if user.AccountID != "" {
			pk = user.AccountID
		} else {
			pk = hash.Values("users", customerID, user.Key, "jira")
		}
	*/
	if s.exported[pk] {
		return nil
	}
	//s.integration.logger.Info("exporting user", "user", user.EmailAddress)
	s.exported[pk] = true
	return s.sendUser(&work.User{
		//ID:         hash.Values(customerID, pk),
		RefType:    "jira",
		RefID:      pk,
		CustomerID: customerID,
		Name:       user.DisplayName,
		Username:   user.Name,
		AvatarURL:  &user.Avatars.Large,
		Email:      &user.EmailAddress,
	})
}

func (s *Users) sendUser(user *work.User) error {
	return s.sendUsers([]*work.User{user})
}

func (s *Users) sendUsers(users []*work.User) error {
	var batch []objsender.Model
	for _, user := range users {
		batch = append(batch, user)
	}
	return s.sender.Send(batch)
}

func (s *Users) Done() {
	s.sender.Done()
}
