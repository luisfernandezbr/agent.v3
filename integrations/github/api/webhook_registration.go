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
	"time"

	"github.com/pinpt/agent/pkg/requests2"
	pstrings "github.com/pinpt/go-common/strings"
)

// update this using current time, if the format of the url changes, or need a new url for some other reason
const webhookReplaceOlderThan = "2020-05-04T17:19:42Z"

func WebhookCreateIfNotExists(qc QueryContext, repo Repo, webhookURL string, events []string) (rerr error) {
	logger := qc.Logger.With("repo", repo.NameWithOwner, "events", events)

	logger.Debug("checking if webhook registration is needed")

	webhooks, noPermissions, err := WebhookList(qc, repo)
	if err != nil {
		rerr = err
		return
	}
	if noPermissions {
		rerr = errors.New("no persmissions to list webhooks for repo")
		return
	}

	webhookReplaceOlderThan, err := time.Parse(time.RFC3339, webhookReplaceOlderThan)
	if err != nil {
		rerr = fmt.Errorf("invalid webhookReplaceOlderThan constant format: %v", err)
		return
	}

	found := false

	for _, wh := range webhooks {
		wantedURL, err := url.Parse(webhookURL)
		if err != nil {
			rerr = err
			return
		}
		haveURL, err := url.Parse(wh.Config.URL)
		if err != nil {
			rerr = err
			return
		}
		if wantedURL.Host == haveURL.Host {
			found = true

			// already exists
			if wh.CreatedAt.Before(webhookReplaceOlderThan) {
				logger.Info("recreating webhook, because the one we had before is older than", "deadline", webhookReplaceOlderThan)

				// the hook was created by older version of agent and needs re-creating
			} else if reflect.DeepEqual(sortCopy(events), sortCopy(wh.Events)) {
				// already same events, nothing to do
				logger.Debug("existing webhook found, no need to re-create")
			} else {
				// remove previous, and add new
				logger.Info("recreating webhook, because the one we had before had different settings", "repo", repo.NameWithOwner)
			}

			// if there are multiple hooks for event-api url, we will remove all previous, only keeping one
			err := webhookRemove(qc, repo, wh.ID)
			if err != nil {
				rerr = err
				return
			}

		}
	}

	if !found {
		logger.Info("existing webhook not found, creating")
	}

	return webhookCreate(qc, repo, webhookURL, events)
}

func sortCopy(arr []string) []string {
	arr2 := make([]string, len(arr))
	copy(arr2, arr)
	sort.Strings(arr2)
	return arr2
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
	reqs := requests2.New(qc.Logger, qc.Clients.TLSInsecure)

	req := requests2.Request{}
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

	reqs := requests2.New(qc.Logger, qc.Clients.TLSInsecure)

	req := requests2.Request{}
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

	reqs := requests2.New(qc.Logger, qc.Clients.TLSInsecure)

	req := requests2.Request{}
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
