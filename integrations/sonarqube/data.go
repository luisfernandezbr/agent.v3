package main

import (
	"github.com/pinpt/agent.next/integrations/pkg/objsender2"
	"github.com/pinpt/integration-sdk/codequality"
)

func (s *Integration) exportAll() error {
	projects, err := s.api.FetchProjects()
	if err != nil {
		s.logger.Error("error fetching projects", "err", err)
		return err
	}
	session, err := objsender2.Root(s.agent, codequality.ProjectModelName.String())
	if err != nil {
		s.logger.Error("error creating project session", "err", err)
		return err
	}
	if err := session.SetTotal(len(projects)); err != nil {
		s.logger.Error("error setting total projects on exportAll", "err", err)
		return err
	}
	for _, project := range projects {
		project.CustomerID = s.customerID
		if err := session.Send(project); err != nil {
			s.logger.Error("error sending project to agent", "err", err, "id", project.RefID)
			return err
		}
		metricsession, err := session.Session(codequality.MetricModelName.String(), project.RefID, project.Name)
		if err != nil {
			s.logger.Error("error creating metric session", "err", err)
			return err
		}
		metrics, err := s.api.FetchMetrics(project, session.LastProcessedTime())
		if err != nil {
			s.logger.Error("error fetching metrics", "err", err)
			return err
		}
		for _, metric := range metrics {
			metric.CustomerID = s.customerID
			if err := metricsession.Send(metric); err != nil {
				s.logger.Error("error sending metric to agent", "err", err, "id", metric.RefID)
				return err
			}
		}
		if err := metricsession.Done(); err != nil {
			return err
		}
	}
	return session.Done()
}
