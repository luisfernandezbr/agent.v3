package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"

	"github.com/pinpt/agent/pkg/requests2"
	pstrings "github.com/pinpt/go-common/strings"
)

func WebhookCreateIfNotExists(qc QueryContext, repo Repo, webhookURL string, events []string) (rerr error) {
	qc.Logger.Debug("checking if webhook registration is needed", "repo", repo.NameWithOwner, "events", events)

	webhooks, err := webhookList(qc, repo)
	if err != nil {
		rerr = err
		return
	}

	for _, wh := range webhooks {
		wantedURL, err := url.Parse(webhookURL)
		if err != nil {
			rerr = err
			return
		}
		haveURL, err := url.Parse(wh.URL)
		if err != nil {
			rerr = err
			return
		}
		if wantedURL.Host == haveURL.Host {
			// already exists
			if reflect.DeepEqual(sortCopy(events), sortCopy(wh.Events)) {
				// already same events, nothing to do
				return nil
			}
			// remove previous, and add new
			qc.Logger.Info("recreating webhook, because the one we had before had different settings", "repo", repo.NameWithOwner)
			err := webhookRemove(qc, repo, wh.ID)
			if err != nil {
				rerr = err
				return
			}
			// no return here, fall down to create
		}
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
	ID     int
	URL    string
	Events []string
}

func webhookList(qc QueryContext, repo Repo) (res []webhook, rerr error) {
	reqs := requests2.New(qc.Logger, qc.Clients.TLSInsecure)

	req := requests2.Request{}
	req.Method = "GET"
	req.URL = pstrings.JoinURL(qc.APIURL3, "repos", repo.NameWithOwner, "hooks")
	req.Header = http.Header{}
	req.Header.Set("Authorization", "token "+qc.AuthToken)

	var resp []struct {
		ID     int      `json:"id"`
		Events []string `json:"events"`
		Config struct {
			URL string `json:"url"`
		} `json:"config"`
	}
	_, err := reqs.JSON(req, &resp)
	if err != nil {
		rerr = err
		return
	}
	for _, obj := range resp {
		obj2 := webhook{
			ID:     obj.ID,
			URL:    obj.Config.URL,
			Events: obj.Events,
		}
		res = append(res, obj2)
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
