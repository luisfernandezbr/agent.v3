package jiracommon

import (
	"sync"

	"github.com/pinpt/agent.next/integrations/pkg/jiracommonapi"
	"github.com/pinpt/agent.next/integrations/pkg/objsender"
	"github.com/pinpt/agent.next/pkg/ids"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/integration-sdk/work"
)

type Users struct {
	sender     *objsender.Session
	exported   map[string]bool
	exportedMu sync.Mutex
	customerID string
}

func NewUsers(customerID string, agent rpcdef.Agent) (_ *Users, rerr error) {
	s := &Users{}
	s.customerID = customerID
	var err error
	s.sender, err = objsender.Root(agent, work.UserModelName.String())
	if err != nil {
		rerr = err
		return
	}
	s.exported = map[string]bool{}
	return s, nil
}

// Export user is safe for concurrent use
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
	s.exportedMu.Lock()
	if s.exported[pk] {
		s.exportedMu.Unlock()
		return nil
	}

	s.exported[pk] = true
	s.exportedMu.Unlock()

	u := &work.User{}
	u.RefType = "jira"
	u.RefID = pk
	u.CustomerID = customerID
	u.Name = user.DisplayName
	u.Username = user.Name
	u.AvatarURL = &user.Avatars.Large
	u.Email = &user.EmailAddress
	if user.Name != "" {
		v := ids.WorkUserAssociatedRefID(customerID, "jira", user.Name)
		u.AssociatedRefID = &v
	}
	return s.sendUser(u)
}

func (s *Users) sendUser(user *work.User) error {
	return s.sendUsers([]*work.User{user})
}

func (s *Users) sendUsers(users []*work.User) error {
	for _, user := range users {
		err := s.sender.Send(user)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Users) Done() error {
	return s.sender.Done()
}
