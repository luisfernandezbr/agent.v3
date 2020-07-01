package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pinpt/agent/pkg/requests"
	pstrings "github.com/pinpt/go-common/strings"
)

// update this using current time, if the format of the url changes, or need a new url for some other reason
const WebhookReplaceOlderThan = "2020-05-04T17:19:42Z"

func WebhookCreateIfNotExists(qc QueryContext, repo Repo, webhookURL string, events []string, webhookReplaceOlderThan string) (rerr error) {
	logger := qc.Logger.With("repo", repo.NameWithOwner, "events", events)

	logger.Debug("checking if webhook registration is needed")

	webhooks, noPermissions, err := WebhookList(qc, repo)
	if err != nil {
		rerr = err
		return
	}
	if noPermissions {
		rerr = errors.New("no permissions to list webhooks for repo")
		return
	}

	webhookReplaceOlder, err := time.Parse(time.RFC3339, webhookReplaceOlderThan)
	if err != nil {
		rerr = fmt.Errorf("invalid webhookReplaceOlderThan constant format: %v", err)
		return
	}

	wantedURL, err := url.Parse(webhookURL)
	if err != nil {
		rerr = err
		return
	}

	var pinptWebHooks []webhook

	for _, wh := range webhooks {
		haveURL, err := url.Parse(wh.Config.URL)
		if err != nil {
			rerr = err
			return
		}
		if strings.Contains(wh.Config.URL, "?integration_instance_id") {
			continue
		}
		if wantedURL.Host == haveURL.Host {
			pinptWebHooks = append(pinptWebHooks, wh)
		}
	}

	whCount := len(pinptWebHooks)

	if whCount == 0 {
		return webhookCreate(qc, repo, webhookURL, events)
	} else if whCount > 1 {

		sort.SliceStable(pinptWebHooks, func(i, j int) bool {
			return pinptWebHooks[i].CreatedAt.Unix() > pinptWebHooks[j].CreatedAt.Unix()
		})

		for _, wh := range pinptWebHooks[1:] {
			if strings.Contains(wh.Config.URL, "?integration_instance_id") {
				continue
			}
			err := webhookRemove(qc, repo, wh.ID)
			if err != nil {
				rerr = err
				return
			}
		}
	}

	wh := pinptWebHooks[0]
	var update bool
	if wh.CreatedAt.Before(webhookReplaceOlder) {
		logger.Info("recreating webhook, because the one we had before is older than", "deadline", webhookReplaceOlderThan)
		update = true
	}
	if !reflect.DeepEqual(pstrings.SortCopy(events), pstrings.SortCopy(wh.Events)) {
		logger.Info("recreating webhook, because the one we had before had different settings", "repo", repo.NameWithOwner)
		update = true
	}

	if update {
		err := webhookRemove(qc, repo, wh.ID)
		if err != nil {
			rerr = err
			return
		}
		err = webhookCreate(qc, repo, webhookURL, events)
		if err != nil {
			rerr = err
			return
		}
	}

	return
}

type webhook struct {
	ID     int      `json:"id"`
	Events []string `json:"events"`
	Config struct {
		URL string `json:"url"`
	} `json:"config"`
	CreatedAt time.Time `json:"created_at"`
}

func WebhookList(qc QueryContext, repo Repo) (res []webhook, noPermissions bool, rerr error) {
	reqs := requests.New(qc.Logger, qc.Clients.TLSInsecure)

	req := requests.Request{}
	req.Method = "GET"
	req.URL = pstrings.JoinURL(qc.APIURL3, "repos", repo.NameWithOwner, "hooks")
	req.Header = http.Header{}
	req.Header.Set("Authorization", "token "+qc.AuthToken)

	resp, err := reqs.JSON(req, &res)
	if err != nil {
		if resp.Resp.StatusCode == http.StatusNotFound {
			noPermissions = true
			return
		}
		rerr = err
		return
	}
	return
}

func webhookCreate(qc QueryContext, repo Repo, webhookURL string, events []string) (rerr error) {
	qc.Logger.Info("registering webhook for repo", "repo", repo.NameWithOwner, "events", events)

	data := struct {
		Name   string   `json:"name"`
		Active bool     `json:"active"`
		Events []string `json:"events"`
		Config struct {
			URL         string `json:"url"`
			ContentType string `json:"content_type"`
			InsecureSSL string `json:"insecure_ssl"`
		} `json:"config"`
	}{}
	data.Name = "web"
	data.Active = true
	data.Events = events
	data.Config.URL = webhookURL
	data.Config.ContentType = "json"
	data.Config.InsecureSSL = "0"

	reqs := requests.New(qc.Logger, qc.Clients.TLSInsecure)

	req := requests.Request{}
	req.Method = "POST"
	req.URL = pstrings.JoinURL(qc.APIURL3, "repos", repo.NameWithOwner, "hooks")
	req.Header = http.Header{}
	req.Header.Set("Authorization", "token "+qc.AuthToken)

	var err error
	req.Body, err = json.Marshal(data)
	if err != nil {
		rerr = err
		return
	}
	var resp interface{}
	_, err = reqs.JSON(req, &resp)
	if err != nil {
		rerr = err
		return
	}

	return nil
}

func webhookRemove(qc QueryContext, repo Repo, hookID int) error {
	qc.Logger.Info("removing webhook", "repo", repo.NameWithOwner, "hook_id", hookID)

	reqs := requests.New(qc.Logger, qc.Clients.TLSInsecure)

	req := requests.Request{}
	req.Method = "DELETE"
	req.URL = pstrings.JoinURL(qc.APIURL3, "repos", repo.NameWithOwner, "hooks", strconv.Itoa(hookID))
	req.Header = http.Header{}
	req.Header.Set("Authorization", "token "+qc.AuthToken)

	res, err := reqs.Do(context.Background(), req)
	if err != nil {
		return err
	}
	if res.Resp.StatusCode != 204 {
		return res.ErrorContext(errors.New("could not delete webhook wanted status 204"))
	}

	return nil
}
