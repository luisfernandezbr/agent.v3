package main

import (
	"time"

	"github.com/pinpt/agent.next/rpcdef"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) exportAll() {
	s.exportBlames()
	s.exportCommits()
}

func (s *Integration) exportBlames() {

	sessionID, lastProcessed := s.agent.ExportStarted(sourcecode.BlameTable.String())
	defer s.agent.ExportDone(sessionID, time.Now().Format(time.RFC3339))

	s.logger.Info("exporting blames", "lastProcessed", lastProcessed)

	for i := 0; i < 10; i++ {
		rows := []map[string]interface{}{}
		for j := 0; j < 10; j++ {
			row := sourcecode.Blame{}
			row.RepoID = "r1"
			row.Filename = "f"
			row.Language = "go"
			rows = append(rows, row.ToMap())
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
}

func (s *Integration) exportCommits() {
	sessionID, lastProcessed := s.agent.ExportStarted(sourcecode.CommitTable.String())
	defer s.agent.ExportDone(sessionID, time.Now().Format(time.RFC3339))

	s.logger.Info("exporting blames", "lastProcessed", lastProcessed)

	for i := 0; i < 10; i++ {
		rows := []map[string]interface{}{}
		for j := 0; j < 10; j++ {
			row := map[string]interface{}{
				"repo_id": "r1",
				"message": "m",
			}
			rows = append(rows, row)
		}
		var objs []rpcdef.ExportObj
		for _, row := range rows {
			obj := rpcdef.ExportObj{}
			obj.Data = row
			objs = append(objs, obj)
		}
		s.agent.SendExported(sessionID, objs)
	}
}
