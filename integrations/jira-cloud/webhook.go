package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/pinpt/agent/integrations/jira-cloud/api"
	"github.com/pinpt/agent/integrations/jira/common"
	"github.com/pinpt/agent/integrations/jira/commonapi"
	"github.com/pinpt/integration-sdk/work"

	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/rpcdef"
)

var webhookEvents = []string{
	"jira:issue_updated",
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

	err := s.initWithConfig(config, false)
	if err != nil {
		rerr(err)
		return
	}

	var data map[string]interface{}

	err = json.Unmarshal([]byte(body), &data)
	if err != nil {
		rerr(err)
		return
	}

	sessions := objsender.NewSessionsWebhook()

	webhookEvent, _ := data["webhookEvent"]
	switch webhookEvent {
	case "":
		rerr(errors.New("webhookEvent key is not set in webhook object"))
		return
	case "jira:issue_updated":
		issue, ok := data["issue"].(map[string]interface{})
		if !ok {
			rerr(errors.New("missing issue in payload"))
			return
		}
		issueIDIface, _ := issue["id"]
		if issueIDIface == "" {
			rerr(errors.New("missing issue.id in playload"))
			return
		}
		issueID, _ := issueIDIface.(string)
		if issueID == "" {
			rerr(errors.New("missing issue.id in playload"))
			return
		}
		err := s.webhookGetUpdatedIssue(sessions, issueID)
		if err != nil {
			rerr(err)
			return
		}
		res.MutatedObjects = sessions.Data
		return
	default:
		s.logger.Info("skipping webhook with unsupported webhookEvent, this is not in a list of supported webhooks", "webhookEvent", webhookEvent)
		return
	}
}

func (s *Integration) webhookGetUpdatedIssue(sessions *objsender.SessionsWebhook, issueIDOrKey string) error {
	fields, err := api.FieldsAll(s.qc)
	if err != nil {
		return err
	}
	fieldByID := map[string]commonapi.CustomField{}
	for _, f := range fields {
		fieldByID[f.ID] = f
	}
	issueResolver := common.NewIssueResolver(s.qc.Common())

	userSender := sessions.NewSession(work.UserModelName.String())

	users, err := common.NewUsers(s.logger, s.qc.CustomerID, s.agent, s.qc.WebsiteURL, userSender)
	if err != nil {
		return err
	}
	qcCommon := s.qc.Common()
	qcCommon.ExportUser = users.ExportUser

	s.logger.Info("getting issue data", "id", issueIDOrKey)
	obj, err := commonapi.IssueByIDFull(qcCommon, issueIDOrKey, fieldByID, issueResolver.IssueRefIDFromKey)
	if err != nil {
		return err
	}
	session := sessions.NewSession(work.IssueModelName.String())
	session.Send(obj)
	return nil
}
