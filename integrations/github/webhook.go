package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/pinpt/agent/integrations/github/api"
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

func (s *Integration) Webhook(ctx context.Context, dataJSON string, config rpcdef.ExportConfig) (res rpcdef.WebhookResult, _ error) {

	rerr := func(err error) {
		res.Error = err.Error()
		return
	}

	if len(dataJSON) == 0 {
		rerr(errors.New("empty webhook data passed"))
		return
	}

	err := s.initWithConfig(config)
	if err != nil {
		rerr(err)
		return
	}

	s.qc.Request = s.makeRequestNoRetries

	var data map[string]interface{}

	err = json.Unmarshal([]byte(dataJSON), &data)
	if err != nil {
		rerr(err)
		return
	}

	s.users, err = NewUsers(s, true)
	if err != nil {
		rerr(err)
		return
	}
	s.qc.UserLoginToRefID = s.users.LoginToRefID
	s.qc.UserLoginToRefIDFromCommit = s.users.LoginToRefIDFromCommit

	xGithubEvent, _ := data["x-github-event"].(string)
	switch xGithubEvent {
	case "":
		s.logger.Info("dataJSON", "v", dataJSON, "o", data)
		rerr(errors.New("x-github-event key is not provided in webhook object, it is sent in request header and should be added to payload"))
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
		return s.returnUpdatedPRForWebhook(prNodeID)
	case "push":
		obj, ok := data["repository"].(map[string]interface{})
		if !ok {
			rerr(errors.New("missing repository map in payload"))
			return
		}
		repoNodeID, _ := obj["node_id"].(string)
		if repoNodeID == "" {
			rerr(errors.New("missing repository.node_id in payload"))
			return
		}
		fullName, _ := obj["full_name"].(string)
		if fullName == "" {
			rerr(errors.New("missing repository.full_name in payload"))
			return
		}
		defaultBranch, _ := obj["default_branch"].(string)
		if fullName == "" {
			rerr(errors.New("missing repository.default_branch in payload"))
			return
		}
		repo := api.Repo{}
		repo.DefaultBranch = defaultBranch
		repo.ID = repoNodeID
		repo.NameWithOwner = fullName
		err := s.exportGit(repo, nil)
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
