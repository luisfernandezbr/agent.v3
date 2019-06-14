package main

import (
	"github.com/pinpt/agent2/rpcdef"
)

func (s *Integration) exportAll() {
	s.exportBlames()
	s.exportCommits()
}

func (s *Integration) exportBlames() {
	s.logger.Info("exporting blames")

	session := s.agent.ExportStarted("sourcecode.blame")
	defer s.agent.ExportDone(session)

	for i := 0; i < 10; i++ {
		rows := []map[string]interface{}{}
		for j := 0; j < 10; j++ {
			row := map[string]interface{}{
				"repo_id":  "r1",
				"filename": "f",
				"language": "go",
			}
			rows = append(rows, row)
		}
		var objs []rpcdef.ExportObj
		for _, row := range rows {
			obj := rpcdef.ExportObj{}
			obj.Data = row
			objs = append(objs, obj)
		}
		s.agent.SendExported(
			session,
			"last_processed_todo",
			objs)
	}
}

func (s *Integration) exportCommits() {
	s.logger.Info("exporting commits")
	session := s.agent.ExportStarted("sourcecode.commit")
	defer s.agent.ExportDone(session)

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
		s.agent.SendExported(session, "last_processed_todo", objs)
	}
}
