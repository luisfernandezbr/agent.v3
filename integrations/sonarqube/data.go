package main

import (
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/integration-sdk/codequality"
)

func (s *Integration) validate() (bool, error) {
	return s.api.Validate()
}

func (s *Integration) exportAll() error {
	var err error
	if err = s.exportProjects(); err != nil {
		return err
	}
	if err = s.exportMetrics(); err != nil {
		return err
	}
	return nil
}

func (s *Integration) exportProjects() error {
	sender, err := objsender.NewIncrementalDateBased(s.agent, codequality.ProjectModelName.String())
	if err != nil {
		return err
	}
	projects, err := s.api.FetchProjects(sender.LastProcessed)
	if err != nil {
		return err
	}

	for _, project := range projects {
		project.CustomerID = s.customerID
		if err := sender.Send(project); err != nil {
			return err
		}
	}

	return sender.Done()
}

func (s *Integration) exportMetrics() error {
	sender, err := objsender.NewIncrementalDateBased(s.agent, codequality.MetricModelName.String())
	if err != nil {
		return err
	}
	metrics, err := s.api.FetchAllMetrics(sender.LastProcessed)
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
