package main

import (
	"github.com/pinpt/agent.next/integrations/sonarqube/api"
	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/integration-sdk/codequality"
)

func (s *Integration) validate() (bool, error) {
	return s.api.Validate()
}

func (s *Integration) exportAll() error {
	var projects []*api.Project
	var err error
	if projects, err = s.exportProjects(); err != nil {
		return err
	}
	if err = s.exportMetrics(projects); err != nil {
		return err
	}
	return nil
}

func (s *Integration) exportProjects() ([]*api.Project, error) {
	sender, err := objsender.NewIncrementalDateBased(s.agent, codequality.ProjectModelName.String())
	if err != nil {
		return nil, err
	}
	projects, err := s.api.FetchProjects(sender.LastProcessed)
	if err != nil {
		return nil, err
	}

	for _, project := range projects {
		var proj codequality.Project
		proj.CustomerID = s.customerID
		proj.Identifier = project.Key
		proj.Name = project.Name
		proj.RefID = project.ID
		proj.RefType = "sonarqube"
		sender.SendMap(proj.ToMap())
	}

	return projects, sender.Done()
}

func (s *Integration) exportMetrics(projects []*api.Project) error {
	sender, err := objsender.NewIncrementalDateBased(s.agent, codequality.MetricModelName.String())
	if err != nil {
		return err
	}
	metrics, err := s.api.FetchAllMetrics(projects, sender.LastProcessed)
	for _, metric := range metrics {
		var metr codequality.Metric
		metr.CustomerID = s.customerID
		metr.Name = metric.Metric
		metr.ProjectID = metric.ProjectID
		metr.RefID = metric.ID
		metr.RefType = "sonarqube"
		metr.Value = metric.Value
		date.ConvertToModel(metric.Date, &metr.CreatedDate)
		sender.SendMap(metr.ToMap())
	}
	return sender.Done()
}
