package main

import (
	"github.com/pinpt/agent.next/integrations/pkg/objsender2"
	"github.com/pinpt/integration-sdk/codequality"
)

func (s *Integration) exportAll() error {
	session, err := objsender2.RootTracking(s.agent, "organization")
	if err != nil {
		s.logger.Error("error creating root tracking for sonarqube", "err", err)
		return err
	}
	projects, err := s.api.FetchProjects()
	if err != nil {
		s.logger.Error("error fetching projects", "err", err)
		return err
	}
	projsession, err := session.Session(codequality.ProjectModelName.String(), "root", "root")
	if err != nil {
		s.logger.Error("error creating project session", "err", err)
		return err
	}
	projsession.SetTotal(len(projects))
	for _, project := range projects {
		project.CustomerID = s.customerID
		if err := projsession.Send(project); err != nil {
			s.logger.Error("error sending project to agent", "err", err)
			continue
		}
		metricsession, err := session.Session(codequality.MetricModelName.String(), project.RefID, project.Name)
		if err != nil {
			s.logger.Error("error creating metric session", "err", err)
			continue
		}
		metrics, err := s.api.FetchMetrics(project, session.LastProcessedTime())
		if err != nil {
			metricsession.Done()
			s.logger.Error("error fetching metrics", "err", err)
			continue
		}
		metricsession.SetTotal(len(metrics))
		for _, metric := range metrics {
			metric.CustomerID = s.customerID
			metricsession.Send(metric)
		}
		metricsession.Done()
	}
	projsession.Done()
	return session.Done()
}
