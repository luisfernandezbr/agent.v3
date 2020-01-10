package main

import (
	"strconv"
	"time"

	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) exportRepoObjects() (repos []rpcdef.ExportProject) {

	sessionID, lastProcessed := s.agent.ExportStarted(sourcecode.RepoModelName.String())
	defer s.agent.ExportDone(sessionID, time.Now().Format(time.RFC3339))

	s.logger.Info("exporting repos", "lastProcessed", lastProcessed)
	c := 0

	for i := 0; i < 2; i++ {
		rows := []map[string]interface{}{}
		for j := 0; j < 2; j++ {
			c++
			n := strconv.Itoa(c)
			row := sourcecode.Repo{}
			row.RefType = "mock"
			row.RefID = "r" + n
			row.Name = "Repo: " + n
			rows = append(rows, row.ToMap())

			repo := rpcdef.ExportProject{}
			repo.ID = row.GetID()
			repo.RefID = row.RefID
			repo.ReadableID = row.RefID
			repo.Error = ""
			repos = append(repos, repo)
		}
		var objs []rpcdef.ExportObj
		for _, row := range rows {
			obj := rpcdef.ExportObj{}
			obj.Data = row
			objs = append(objs, obj)
		}
		s.agent.SendExported(
			sessionID,
			objs)

	}

	return
}
