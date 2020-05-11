package main

import (
	"net/url"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/gitlab/api"
	"github.com/pinpt/agent/integrations/pkg/repoprojects"
	"github.com/pinpt/integration-sdk/work"
)

func (s *Integration) exportWorkIssues(ctx *repoprojects.ProjectCtx, proj repoprojects.RepoProject, usermap api.UsernameMap) error {

	commentChan := make(chan []work.IssueComment)
	errChan := make(chan error)
	go func() {
		commentSender, err := ctx.Session(work.IssueCommentModelName)
		if err != nil {
			errChan <- err
			return
		}
		for comments := range commentChan {
			for _, comment := range comments {
				err := commentSender.Send(&comment)
				if err != nil {
					errChan <- err
					return
				}
			}
		}
		errChan <- nil
	}()

	projectSender, err := ctx.Session(work.IssueModelName)
	lastUpdated := projectSender.LastProcessedTime()
	err = api.PaginateStartAt(s.logger, func(log hclog.Logger, paginationParams url.Values) (page api.PageInfo, _ error) {

		if !lastUpdated.IsZero() {
			paginationParams.Set("updated_after", lastUpdated.Format(time.RFC3339Nano))
		}
		pi, res, err := api.WorkIssuesPage(s.qc, proj.GetID(), usermap, commentChan, paginationParams)
		if err != nil {
			return pi, err
		}

		if err = projectSender.SetTotal(pi.Total); err != nil {
			return pi, err
		}
		for _, obj := range res {
			err := projectSender.Send(obj)
			if err != nil {
				return pi, err
			}
		}
		return pi, nil
	})
	close(commentChan)
	if err != nil {
		return err
	}
	return <-errChan
}
