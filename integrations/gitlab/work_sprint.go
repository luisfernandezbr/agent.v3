package main

import (
	"net/url"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/gitlab/api"
	"github.com/pinpt/agent/integrations/pkg/repoprojects"
	"github.com/pinpt/integration-sdk/work"
)

func (s *Integration) exportWorkSprints(ctx *repoprojects.ProjectCtx, proj repoprojects.RepoProject) error {

	projectSender, err := ctx.Session(work.SprintModelName)
	lastUpdated := projectSender.LastProcessedTime()
	err = api.PaginateStartAt(s.logger, func(log hclog.Logger, paginationParams url.Values) (page api.PageInfo, _ error) {

		if !lastUpdated.IsZero() {
			paginationParams.Set("updated_after", lastUpdated.Format(time.RFC3339Nano))
		}
		pi, res, err := api.WorkSprintPage(s.qc, proj.GetID(), paginationParams)
		if err != nil {
			return pi, err
		}

		if err = projectSender.SetTotal(pi.Total); err != nil {
			return pi, err
		}
		for _, obj := range res {
			s.logger.Info("sending sprint", "sprint", obj.RefID)
			err := projectSender.Send(obj)
			if err != nil {
				return pi, err
			}
		}
		return pi, nil
	})
	if err != nil {
		return err
	}
	return nil
}
