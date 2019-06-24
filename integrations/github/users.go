package main

import (
	"context"

	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/go-datamodel/sourcecode"
)

func (s *Integration) exportUsers(ctx context.Context) error {
	et, err := s.newExportType("sourcecode.User")
	if err != nil {
		return err
	}
	defer et.Done()

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
			batch = append(batch, rpcdef.ExportObj{Data: user.ToMap()})
		}
	}
	if len(batch) == 0 {
		return nil
	}

	s.agent.SendExported(et.SessionID, batch)

	return nil
}
