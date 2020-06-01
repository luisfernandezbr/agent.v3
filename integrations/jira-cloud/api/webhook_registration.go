package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/pinpt/agent/pkg/requests"
	"github.com/pinpt/go-common/datetime"

	pstrings "github.com/pinpt/go-common/strings"
)

// update this using current time, if the format of the url changes, or need a new url for some other reason
const webhookReplaceOlderThan = "2020-05-29T02:02:42Z"

func WebhookCreateIfNotExists(qc QueryContext, webhookURL string, events []string) (rerr error) {

	qc.Logger.Debug("checking if webhook registration is needed")

	webhooks, noPermissions, err := webhookList(qc)
	if err != nil {
		rerr = err
		return
	}
	if noPermissions {
		rerr = errors.New("no permissions to list webhooks")
		return
	}

	webhookReplaceOlderThan, err := time.Parse(time.RFC3339, webhookReplaceOlderThan)
	if err != nil {
		rerr = fmt.Errorf("invalid webhookReplaceOlderThan constant format: %v", err)
		return
	}

	getLastEventAPIWebHook := func() (*webhook, error) {

		var lastWebHook *webhook

		wantedURL, err := url.Parse(webhookURL)
		if err != nil {
			return nil, err
		}

		for _, wh := range webhooks {

			haveURL, err := url.Parse(wh.URL)
			if err != nil {
				return nil, err
			}

			if strings.Contains(wantedURL.Host, haveURL.Host) {
				if lastWebHook == nil {
					lastWebHook = &wh
					continue
				}
				var whForDeletetion *webhook
				if wh.LastUpdated > lastWebHook.LastUpdated {
					whForDeletetion = lastWebHook
					lastWebHook = &wh
				} else {
					whForDeletetion = &wh
				}
				err := WebhookRemove(qc, whForDeletetion.Self)
				if err != nil {
					return nil, err
				}
			}

		}

		return lastWebHook, nil
	}

	wh, err := getLastEventAPIWebHook()
	if err != nil {
		return err
	}

	if wh != nil {
		createdAt := datetime.DateFromEpoch(wh.LastUpdated)
		// already exists
		if createdAt.Before(webhookReplaceOlderThan) {
			qc.Logger.Info("updating webhook, because the one we had before is older than", "deadline", webhookReplaceOlderThan)
			return webhookUpdate(qc, webhookURL, wh.Self, events)
		}
		// the hook was created by older version of agent and needs re-creating
		if reflect.DeepEqual(pstrings.SortCopy(events), pstrings.SortCopy(wh.Events)) {
			// already same events, nothing to do
			qc.Logger.Debug("existing webhook found, no need to re-create")
			return
		}

		return webhookUpdate(qc, webhookURL, wh.Self, events)
	}

	qc.Logger.Info("webhook not found, creating")
	return WebhookCreate(qc, webhookURL, events)
}

type webhook struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	ExcludeBody bool   `json:"excludeBody"`
	Filters     struct {
		IssueRelatedEventsSection string `json:"issue-related-events-section"`
	} `json:"filters"`
	Events                 []string `json:"events"`
	Enabled                bool     `json:"enabled"`
	Self                   string   `json:"self"`
	LastUpdatedUser        string   `json:"lastUpdatedUser"`
	LastUpdatedDisplayName string   `json:"lastUpdatedDisplayName"`
	LastUpdated            int64    `json:"lastUpdated"`
}

func webhookList(qc QueryContext) (res []webhook, noPermissions bool, rerr error) {

	qc.Logger.Info("getting webhook list")

	reqs := requests.New(qc.Logger, qc.Clients.TLSInsecure)

	url := getWebHooksURL(qc)

	req := buildBasicRequest(qc, "GET", url)

	resp, err := reqs.JSON(req, &res)
	if err != nil {
		if resp.Resp.StatusCode == http.StatusUnauthorized {
			noPermissions = true
			return
		}
		rerr = err
		return
	}
	return
}

func WebhookCreate(qc QueryContext, webhookURL string, events []string) error {
	qc.Logger.Info("creating webhook")

	url := getWebHooksURL(qc)

	_, err := webhookRequest(qc, "POST", webhookURL, url, events, nil)

	return err

}

func WebhookTestPermissions(qc QueryContext) (webhookURL string, noPermissions bool, rerr error) {
	qc.Logger.Info("testing webhook permissions")

	url := getWebHooksURL(qc)

	var wh webhook

	resp, err := webhookRequest(qc, "POST", "https://example.com", url, []string{"jira:issue_created"}, &wh)
	if err != nil {
		rerr = err
		if resp.Resp.StatusCode == http.StatusUnauthorized ||
			resp.Resp.StatusCode == http.StatusForbidden {
			noPermissions = true
		}
		return
	}

	webhookURL = wh.Self

	return

}

func webhookUpdate(qc QueryContext, webhookURL string, webhookSelf string, events []string) error {
	qc.Logger.Info("updating webhook")

	_, err := webhookRequest(qc, "PUT", webhookURL, webhookSelf, events, nil)

	return err
}

func WebhookRemove(qc QueryContext, endPointURL string) (rerr error) {

	reqs := requests.New(qc.Logger, qc.Clients.TLSInsecure)

	qc.Logger.Info("deleting webhook", "webhook", endPointURL)

	req := buildBasicRequest(qc, "DELETE", endPointURL)

	res, err := reqs.Do(context.Background(), req)
	if err != nil {
		return err
	}

	if res.Resp.StatusCode != 204 {
		return res.ErrorContext(errors.New("could not delete webhook wanted status 204"))
	}

	return nil
}

func buildBasicRequest(qc QueryContext, method, endpoint string) requests.Request {
	return requests.Request{
		Method:            method,
		URL:               endpoint,
		BasicAuthUser:     qc.User,
		BasicAuthPassword: qc.ApiToken,
	}
}

func getWebHooksURL(qc QueryContext) string {
	return pstrings.JoinURL(qc.WebsiteURL, "rest/webhooks/1.0/webhook")
}

func webhookRequest(qc QueryContext, method string, webhookURL string, endPointURL string, events []string, resp interface{}) (res requests.Result, rerr error) {

	data := struct {
		URL    string   `json:"url"`
		Name   string   `json:"name"`
		Events []string `json:"events"`
	}{}

	data.Name = "web"
	// jira api doesn't allow port numbers in the URL
	// we need to delete 8443 port for local tests
	re := regexp.MustCompile(`:[0-9]+`)
	data.URL = re.ReplaceAllString(webhookURL, "")
	data.Events = events

	reqs := requests.New(qc.Logger, qc.Clients.TLSInsecure)

	req := buildBasicRequest(qc, method, endPointURL)

	var err error
	req.Body, err = json.Marshal(data)
	if err != nil {
		rerr = err
		return
	}
	if resp == nil {
		resp = new(interface{})
	}
	return reqs.JSON(req, &resp)
}
