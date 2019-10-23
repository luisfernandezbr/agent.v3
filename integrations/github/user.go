package main

import (
	"errors"
	"strings"
	"sync"

	"github.com/pinpt/agent.next/integrations/pkg/objsender"

	pstrings "github.com/pinpt/go-common/strings"

	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/integration-sdk/sourcecode"
)

// map[login]refID
type Users struct {
	integration *Integration
	sender      *objsender.Session
	loginToID   map[string]string

	mu sync.Mutex
}

func NewUsers(integration *Integration, orgs []api.Org) (*Users, error) {
	s := &Users{}
	s.integration = integration
	var err error
	s.sender, err = objsender.Root(integration.agent, sourcecode.UserModelName.String())
	if err != nil {
		return nil, err
	}
	s.loginToID = map[string]string{}

	err = s.createGhost()
	if err != nil {
		return nil, err
	}

	if integration.config.Enterprise {
		err = s.exportInstanceUsers()
		if err != nil {
			return nil, err
		}
	} else {
		for _, org := range orgs {
			err = s.exportOrganizationUsers(org)
			if err != nil {
				return nil, err
			}
		}
	}

	return s, nil
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

	s.loginToID[""] = "ghost"

	user := &sourcecode.User{}
	user.RefID = "ghost"
	user.RefType = "github"
	user.Name = "Ghost (all deleted users)"
	user.Username = pstrings.Pointer("ghost")
	user.Member = false
	user.Type = sourcecode.UserTypeDeletedSpecialUser
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

func (s *Users) LoginToRefID(login string) (refID string, _ error) {
	if login == "dependabot" || strings.HasSuffix(login, "[bot]") {
		return "", nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loginToID[login] == "" {
		s.integration.logger.Info("could not find user in organization querying non-org github user", "login", login)
		user, err := api.User(s.integration.qc, login, false)
		if err != nil {
			return "", err
		}
		err = s.sendUser(user)
		if err != nil {
			return "", err
		}
		s.loginToID[*user.Username] = user.RefID
	}
	return s.loginToID[login], nil
}

func (s *Users) LoginToRefIDFromCommit(login string, email string) (refID string, _ error) {
	if email == "noreply@github.com" {
		return "", errors.New("can not resolve user with email: noreply@github.com")
	}
	return s.LoginToRefID(login)
}

func (s *Users) Done() error {
	return s.sender.Done()
}
