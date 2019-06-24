package main

import (
	"fmt"
	"time"

	"github.com/pinpt/agent.next/integrations/github/api"
	"github.com/pinpt/agent.next/rpcdef"
)

type exportType struct {
	RefType   string
	SessionID string

	integration *Integration

	lastProcessed time.Time

	startTime time.Time
}

func (s *Integration) newExportType(refType string) (*exportType, error) {
	res := &exportType{}
	res.RefType = refType
	res.integration = s
	return res, res.init()
}

func (s *exportType) init() error {
	s.startTime = time.Now()

	sessionID, lastProcessedData := s.integration.agent.ExportStarted(s.RefType)

	s.SessionID = sessionID
	if lastProcessedData != nil {
		var err error
		s.lastProcessed, err = time.Parse(time.RFC3339, lastProcessedData.(string))
		if err != nil {
			return fmt.Errorf("last processed timestamp is not valid, err: %v", err)
		}
	}

	return nil
}

func (s *exportType) Paginate(defaultOrderIsByUpdatedAt bool, fn api.PaginateNewerThanFn) error {
	return api.PaginateNewerThan(s.lastProcessed, fn, defaultOrderIsByUpdatedAt)
}

func (s *exportType) Send(objs []rpcdef.ExportObj) error {
	if len(objs) == 0 {
		return nil
	}
	s.integration.agent.SendExported(s.SessionID, objs)
	return nil
}

func (s *exportType) Done() {
	s.integration.agent.ExportDone(s.SessionID, s.startTime.Format(time.RFC3339))
}
