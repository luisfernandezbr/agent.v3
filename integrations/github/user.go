package main

import (
	"errors"
	"sync"

	pstrings "github.com/pinpt/go-common/strings"

	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/go-datamodel/sourcecode"
)

// map[login]refID
type Users struct {
	integration *Integration
	et          *exportType
	loginToID   map[string]string

	mu sync.Mutex
}

func NewUsers(integration *Integration) (*Users, error) {
	s := &Users{}
	s.integration = integration
	var err error
	s.et, err = s.integration.newExportType("sourcecode.user")
	if err != nil {
		return nil, err
	}
	s.loginToID = map[string]string{}

	err = s.createGhost()
	if err != nil {
		return nil, err
	}

	err = s.exportOrganizationUsers()
	if err != nil {
		return nil, err
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

	user := sourcecode.User{}
	user.RefID = "ghost"
	user.RefType = "github"
	user.Name = "Ghost (all deleted users)"
	user.Username = pstrings.Pointer("ghost")
	user.Member = false
	user.Type = sourcecode.TypeDeletedSpecialUser
	return s.sendUser(user)
}

func (s *Users) sendUser(user sourcecode.User) error {
	return s.sendUsers([]sourcecode.User{user})
}

func (s *Users) sendUsers(users []sourcecode.User) error {
	batch := []rpcdef.ExportObj{}
	for _, user := range users {
		batch = append(batch, rpcdef.ExportObj{Data: user.ToMap()})
	}
	return s.et.Send(batch)
}

func (s *Users) exportOrganizationUsers() error {
	resChan := make(chan []sourcecode.User)

	go func() {
		defer close(resChan)
		err := api.UsersAll(s.integration.qc, resChan)
		if err != nil {
			panic(err)
		}
	}()

	batch := []rpcdef.ExportObj{}
	for users := range resChan {
		for _, user := range users {
			s.loginToID[*user.Username] = user.RefID
			batch = append(batch, rpcdef.ExportObj{Data: user.ToMap()})
		}
	}
	if len(batch) == 0 {
		return errors.New("no users found, will not be able to map logins to ids")
	}

	s.integration.agent.SendExported(s.et.SessionID, batch)
	return nil
}

func (s *Users) LoginToRefID(login string) (refID string, _ error) {
	if login == "dependabot" || login == "dependabot[bot]" {
		// TODO: to handle bots we will need to track urls
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
		batch := []rpcdef.ExportObj{}
		batch = append(batch, rpcdef.ExportObj{Data: user.ToMap()})
		s.integration.agent.SendExported(s.et.SessionID, batch)
		s.loginToID[*user.Username] = user.RefID
	}
	return s.loginToID[login], nil
}

func (s *Users) LoginToRefIDFromCommit(login string, email string) (refID string, _ error) {
	if email == "noreply@github.com" {
		panic("email:" + email + "login:" + login)
		return "", nil
	}
	return s.LoginToRefID(login)
}

func (s *Users) Done() {
	s.et.Done()
}
