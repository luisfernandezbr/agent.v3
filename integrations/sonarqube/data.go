package main

import (
	"fmt"
	"strings"

	"github.com/pinpt/agent.next/integrations/pkg/objsender2"
	"github.com/pinpt/integration-sdk/codequality"
)

func (s *Integration) exportAll() error {
	session, err := objsender2.Root(s.agent, "sonarqube")
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
	if err := projsession.SetTotal(len(projects)); err != nil {
		s.logger.Error("error setting total projects on exportAll", "err", err)
	}
	var errors []string
	for _, project := range projects {
		project.CustomerID = s.customerID
		if err := projsession.Send(project); err != nil {
			s.logger.Error("error sending project to agent", "err", err, "id", project.RefID)
			continue
		}
		metricsession, err := session.Session(codequality.MetricModelName.String(), project.RefID, project.Name)
		if err != nil {
			s.logger.Error("error creating metric session", "err", err)
			continue
		}
		metrics, err := s.api.FetchMetrics(project, session.LastProcessedTime())
		if err != nil {
			if err := metricsession.Done(); err != nil {
				errors = append(errors, err.Error())
			}
			s.logger.Error("error fetching metrics", "err", err)
			continue
		}
		for _, metric := range metrics {
			metric.CustomerID = s.customerID
			if err := metricsession.Send(metric); err != nil {
				s.logger.Error("error sending metric to agent", "err", err, "id", metric.RefID)
			}
		}
		if err := metricsession.Done(); err != nil {
			errors = append(errors, err.Error())
		}
	}
	if err := projsession.Done(); err != nil {
		errors = append(errors, err.Error())
	}
	if err := session.Done(); err != nil {
		errors = append(errors, err.Error())
	}
	if errors != nil {
		return fmt.Errorf("error with session done. %v", strings.Join(errors, ", "))
	}
	return nil
}
