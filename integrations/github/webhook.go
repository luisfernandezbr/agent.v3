package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hashicorp/go-hclog"

	"github.com/pinpt/agent/integrations/github/api"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func (s *Integration) webhookResult(modelName datamodel.ModelNameType, obj Model) (res rpcdef.WebhookResult, rerr error) {
	objs := rpcdef.MutatedObjects{}
	objs[modelName.String()] = []interface{}{obj.ToMap()}
	res.MutatedObjects = objs
	return
}

func (s *Integration) returnUpdatedPRForWebhook(prRefID string) (res rpcdef.WebhookResult, rerr error) {
	m, err := s.getUpdatedPR(prRefID)
	if err != nil {
		rerr = err
		return
	}
	objs := rpcdef.MutatedObjects{}
	objs[sourcecode.PullRequestModelName.String()] = []interface{}{m}
	res.MutatedObjects = objs
	return
}

var webhookEvents = []string{
	"issue_comment",
	"pull_request",
	"push",
}

func (s *Integration) Webhook(ctx context.Context, headers map[string]string, body string, config rpcdef.ExportConfig) (res rpcdef.WebhookResult, _ error) {

	rerr := func(err error) {
		res.Error = err.Error()
		return
	}

	if len(body) == 0 {
		rerr(errors.New("empty webhook body passed"))
		return
	}

	err := s.initWithConfig(config)
	if err != nil {
		rerr(err)
		return
	}

	s.qc.Request = s.makeRequestNoRetries

	var data map[string]interface{}

	err = json.Unmarshal([]byte(body), &data)
	if err != nil {
		rerr(err)
		return
	}

	sessions := objsender.NewSessionsWebhook()
	s.users, err = NewUsersWebhooks(s, sessions)
	if err != nil {
		rerr(err)
		return
	}
	//s.qc.UserLoginToRefID = s.users.LoginToRefID
	//s.qc.UserLoginToRefIDFromCommit = s.users.LoginToRefIDFromCommit
	s.qc.ExportUserUsingFullDetails = s.users.ExportUserUsingFullDetails

	xGithubEvent, _ := headers["x-github-event"]
	switch xGithubEvent {
	case "":
		rerr(fmt.Errorf("x-github-event key is not provided in webhook object, it is sent in request header and should be added to payload, headers %v", headers))
		return
	case "issue_comment":
		comment, ok := data["comment"].(map[string]interface{})
		if !ok {
			rerr(errors.New("missing comment map in payload"))
			return
		}
		commentNodeID, _ := comment["node_id"].(string)
		if commentNodeID == "" {
			rerr(errors.New("missing comment.node_id in payload"))
			return
		}
		obj, err := api.PullRequestComment(s.qc, commentNodeID)
		if err != nil {
			rerr(err)
			return
		}
		return s.webhookResult(sourcecode.PullRequestCommentModelName, obj)
	case "pull_request":
		repo, err := repoFromWebhook(data)
		if err != nil {
			rerr(err)
			return
		}
		obj, ok := data["pull_request"].(map[string]interface{})
		if !ok {
			rerr(errors.New("missing pull_request map in payload"))
			return
		}
		prNodeID, _ := obj["node_id"].(string)
		if prNodeID == "" {
			rerr(errors.New("missing pull_request.node_id in payload"))
			return
		}
		err = s.webhookPullRequest(s.logger, sessions, repo, prNodeID)
		if err != nil {
			rerr(fmt.Errorf("could not get pull request %v", err))
			return
		}
		res.MutatedObjects = sessions.Data
		return
	case "push":
		repo, err := repoFromWebhook(data)
		if err != nil {
			rerr(err)
			return
		}
		err = s.exportGit(repo, nil)
		if err != nil {
			rerr(err)
			return
		}
		return
	default:
		s.logger.Info("skipping webhook with x-github-event %v, this is not in a list of supported webhooks", xGithubEvent)
		return
	}
}

func repoFromWebhook(data map[string]interface{}) (res api.Repo, rerr error) {
	obj, ok := data["repository"].(map[string]interface{})
	if !ok {
		rerr = errors.New("missing repository map in payload")
		return
	}
	res.ID, _ = obj["node_id"].(string)
	if res.ID == "" {
		rerr = errors.New("missing repository.node_id in payload")
		return
	}
	res.NameWithOwner, _ = obj["full_name"].(string)
	if res.NameWithOwner == "" {
		rerr = errors.New("missing repository.full_name in payload")
		return
	}
	return
}

func (s *Integration) webhookPullRequest(logger hclog.Logger, sessions *objsender.SessionsWebhook, repo api.Repo, prNodeID string) (rerr error) {
	logger = logger.With("repo", repo.NameWithOwner)

	pr, err := api.PullRequestByID(s.qc, prNodeID)
	if err != nil {
		rerr = err
		return
	}

	// export pull request reviews
	pullRequestReviewSender := sessions.NewSession(sourcecode.PullRequestReviewModelName.String())
	err = s.exportPullRequestReviews(logger, pullRequestReviewSender, repo, pr.RefID)
	if err != nil {
		return err
	}

	// export pull request commits
	pullRequestSender := sessions.NewSession(sourcecode.PullRequestModelName.String())
	commitsSender := sessions.NewSession(sourcecode.PullRequestCommitModelName.String())

	return s.exportPRCommitsAddingToPR(logger, repo, pr, pullRequestSender, commitsSender)
}
