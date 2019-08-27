package main

import (
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/integration-sdk/codequality"
)

func (s *Integration) validate() (bool, error) {
	return s.api.Validate()
}

func (s *Integration) exportAll() (err error) {
	var projects []*codequality.Project
	if projects, err = s.exportProjects(); err != nil {
		return
	}
	if err = s.exportMetrics(projects); err != nil {
		return
	}
	return
}

func (s *Integration) exportProjects() (projects []*codequality.Project, err error) {
	sender := objsender.NewNotIncremental(s.agent, codequality.ProjectModelName.String())
	projects, err = s.api.FetchProjects()
	if err != nil {
		return nil, err
	}

	for _, project := range projects {
		project.CustomerID = s.customerID
		if err := sender.Send(project); err != nil {
			return nil, err
		}
	}
	return projects, sender.Done()
}

func (s *Integration) exportMetrics(projects []*codequality.Project) error {
	sender, err := objsender.NewIncrementalDateBased(s.agent, codequality.MetricModelName.String())
	if err != nil {
		return err
	}
	metrics, err := s.api.FetchAllMetrics(projects, sender.LastProcessed)
	if err != nil {
		return err
	}
	for _, metric := range metrics {
		metric.CustomerID = s.customerID
		if err := sender.Send(metric); err != nil {
			return err
		}
	}
	return sender.Done()
}
