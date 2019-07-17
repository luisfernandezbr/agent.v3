package main

import (
	"github.com/pinpt/agent.next/integrations/jira/api"
	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/go-datamodel/work"
)

type Users struct {
	integration *Integration
	sender      *senderNoIncrementals
	exported    map[string]bool
}

func NewUsers(integration *Integration) (*Users, error) {
	s := &Users{}
	s.integration = integration
	s.sender = newSenderNoIncrementals(integration, "work.user")
	s.exported = map[string]bool{}
	return s, nil
}

func (s *Users) ExportUser(user api.User) error {
	s.integration.logger.Info("exporting user", "user", user.EmailAddress)
	customerID := s.integration.qc.CustomerID
	pk := ""
	if user.AccountID != "" {
		pk = user.AccountID
	} else {
		pk = hash.Values("users", customerID, user.Key, "jira")
	}
	if s.exported[pk] {
		return nil
	}
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
	var batch []Model
	for _, user := range users {
		batch = append(batch, user)
	}
	return s.sender.Send(batch)
}

func (s *Users) Done() {
	s.sender.Done()
}
