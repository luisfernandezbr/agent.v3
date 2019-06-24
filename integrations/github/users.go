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
	done := make(chan bool)

	go func() {
		defer func() {
			done <- true
		}()
		batch := []rpcdef.ExportObj{}
		for users := range resChan {
			for _, user := range users {
				batch = append(batch, rpcdef.ExportObj{Data: user.ToMap()})
			}
		}
		if len(batch) == 0 {
			return
		}
		s.agent.SendExported(et.SessionID, batch)
	}()

	err = api.UsersAll(s.qc, resChan)
	<-done
	if err != nil {
		return err
	}
	return nil
}
