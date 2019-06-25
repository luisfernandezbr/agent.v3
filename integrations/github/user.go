package main

import (
	"context"
	"errors"

	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/go-datamodel/sourcecode"
)

func (s *Integration) exportUsers(ctx context.Context) (loginToRefID map[string]string, _ error) {
	et, err := s.newExportType("sourcecode.user")
	if err != nil {
		return nil, err
	}
	defer et.Done()

	loginToRefID = map[string]string{}

	resChan := make(chan []sourcecode.User)

	go func() {
		defer close(resChan)
		err := api.UsersAll(s.qc, resChan)
		if err != nil {
			panic(err)
		}
	}()

	batch := []rpcdef.ExportObj{}
	for users := range resChan {
		for _, user := range users {
			loginToRefID[*user.Username] = user.RefID
			batch = append(batch, rpcdef.ExportObj{Data: user.ToMap()})
		}
	}
	if len(batch) == 0 {
		return nil, errors.New("no users found, will not be able to map logins to ids")
	}

	s.agent.SendExported(et.SessionID, batch)

	return loginToRefID, nil
}
