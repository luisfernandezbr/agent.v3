package main

import (
	"errors"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/pkg/objsender"

	pstrings "github.com/pinpt/go-common/strings"

	"github.com/pinpt/agent/integrations/github/api"
	"github.com/pinpt/integration-sdk/sourcecode"
)

// map[login]refID
type Users struct {
	integration *Integration
	sender      *objsender.Session
	loginToID   map[string]string

	mu sync.Mutex
}

func NewUsers(integration *Integration) (*Users, error) {
	s := &Users{}
	s.integration = integration
	var err error
	s.sender, err = objsender.Root(integration.agent, sourcecode.UserModelName.String())
	if err != nil {
		return nil, err
	}
	s.loginToID = map[string]string{}

	return s, nil
}

func (s *Users) ExportAllOrgUsers(orgs []api.Org) error {
	err := s.createGhost()
	if err != nil {
		return err
	}
	err = s.createGithubNoReply()
	if err != nil {
		return err
	}

	if s.integration.config.Enterprise {
		err = s.exportInstanceUsers()
		if err != nil {
			return err
		}
	} else {
		for _, org := range orgs {
			err = s.exportOrganizationUsers(org)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Users) createGhost() error {
	// create a special deleted user
	// https://github.com/ghost

	/*
		example ghost user
		query {
		repository(owner:"pinpt" name:"worker"){
		pullRequest(number: 79){
		comments(first:10) {
		nodes {
		id
		author {
		login
		url
		}
		}
		}
		}
		}
		}
	*/

	user := &sourcecode.User{}
	user.RefID = "ghost"
	user.RefType = "github"
	user.Name = "Ghost (all deleted users)"
	user.Username = pstrings.Pointer("ghost")
	user.Member = false
	user.Type = sourcecode.UserTypeDeletedSpecialUser
	return s.sendUser(user)
}

func (s *Users) createGithubNoReply() error {
	// commits from github bot are created with noreply@github.com email
	user := &sourcecode.User{}
	user.RefID = "github-noreply"
	user.RefType = "github"
	user.Name = "GitHub (noreply)"
	user.Username = pstrings.Pointer("github")
	user.Member = false
	user.Type = sourcecode.UserTypeBot
	return s.sendUser(user)
}

func (s *Users) sendUser(user *sourcecode.User) error {
	return s.sendUsers([]*sourcecode.User{user})
}

func (s *Users) sendUsers(users []*sourcecode.User) error {
	for _, user := range users {
		err := s.sender.Send(user)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Users) exportInstanceUsers() error {
	resChan := make(chan []*sourcecode.User)
	done := make(chan error)

	go func() {
		err := api.UsersEnterpriseAll(s.integration.qc, resChan)
		close(resChan)
		done <- err
	}()

	err := s.exportUsersFromChan(resChan)
	if err != nil {
		return err
	}

	return <-done
}

func (s *Users) exportUsersFromChan(usersChan chan []*sourcecode.User) error {
	var batch []objsender.Model
	for users := range usersChan {
		for _, user := range users {
			s.loginToID[*user.Username] = user.RefID
			err := s.sender.Send(user)
			if err != nil {
				return err
			}
			batch = append(batch, user)
		}
	}
	if len(batch) == 0 {
		return errors.New("no users found, will not be able to map logins to ids")
	}

	return nil
}

func (s *Users) exportOrganizationUsers(org api.Org) error {
	resChan := make(chan []*sourcecode.User)
	done := make(chan error)

	go func() {
		err := api.UsersAll(s.integration.qc, org, resChan)
		close(resChan)
		done <- err
	}()

	err := s.exportUsersFromChan(resChan)
	if err != nil {
		return err
	}

	return <-done
}

func (s *Users) LoginToRefID(login string) (refID string, rerr error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loginToID[login] != "" {
		return s.loginToID[login], nil
	}

	logger := s.integration.logger

	if login == "" {
		// deleted authors don't have login, but ui links to ghost user
		return "ghost", nil
	}

	if login == "dependabot" || strings.HasSuffix(login, "[bot]") {
		logger.Info("user is a bot, creating new record", "login", login)
		s.loginToID[login] = login

		user := &sourcecode.User{}
		user.RefID = login
		user.RefType = "github"
		user.Name = "Bot " + login
		user.Username = pstrings.Pointer(login)
		user.Member = false
		user.Type = sourcecode.UserTypeBot
		err := s.sendUser(user)
		if err != nil {
			rerr = err
			return
		}

		return login, nil
	}

	logger.Info("could not find user in organization, querying non-org github user for additional details", "login", login)

	user, err := api.User(s.integration.qc, login, false)
	if err != nil {
		rerr = err
		return
	}
	err = s.sendUser(user)
	if err != nil {
		rerr = err
		return
	}
	s.loginToID[*user.Username] = user.RefID
	return s.loginToID[login], nil
}

func (s *Users) LoginToRefIDFromCommit(logger hclog.Logger, login, name, email string) (refID string, _ error) {
	// link all github bot commits
	if name == "GitHub" && email == "noreply@github.com" {
		return "github-noreply", nil
	}
	if login == "" {
		// this is for commits with no matching github accounts, which is completely normaly
		return "", nil
	}
	return s.LoginToRefID(login)
}

func (s *Users) Done() error {
	return s.sender.Done()
}
