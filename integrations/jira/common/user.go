package common

import (
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/jira/commonapi"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/pkg/ids"
	"github.com/pinpt/agent/rpcdef"
	pstrings "github.com/pinpt/go-common/v10/strings"
	"github.com/pinpt/integration-sdk/work"
)

type Users struct {
	logger     hclog.Logger
	sender     objsender.SessionCommon
	exported   map[string]bool
	exportedMu sync.Mutex
	customerID string
	websiteURL string
}

func NewUsers(logger hclog.Logger, customerID string, agent rpcdef.Agent, websiteURL string, sender objsender.SessionCommon) (_ *Users, rerr error) {
	s := &Users{}
	s.logger = logger
	s.customerID = customerID
	s.sender = sender
	s.exported = map[string]bool{}
	s.websiteURL = websiteURL
	return s, nil
}

// Export user is safe for concurrent use
func (s *Users) ExportUser(user commonapi.User) error {
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
	u.Member = user.Active
	if user.Name != "" {
		v := ids.WorkUserAssociatedRefID(customerID, "jira", user.Name)
		u.AssociatedRefID = &v
	}
	if user.AccountID != "" {
		// this is cloud
		u.URL = pstrings.Pointer(s.websiteURL + "/jira/people/" + user.AccountID)
	} else {
		// this is hosted
		// TODO: not sure this actually works, that's the url that links to the user profile,
		// but on our test hosted server it hangs forever when used in jira
		u.URL = pstrings.Pointer(s.websiteURL + "/secure/ViewProfile.jspa?name=" + user.Key)
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
