package api

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/reqstats"
	"github.com/stretchr/testify/assert"
)

func TestWebhookCreateIfNotExists(t *testing.T) {

	assert := assert.New(t)

	firstDate := time.Now()
	secondDateStr := firstDate.Add(time.Second).Format("2006-01-02T15:04:05.999999-07:00")

	// Start a local HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {

		var bts = []byte("{}")

		if req.Method == http.MethodDelete {
			assert.True(
				req.URL.Path == "/repos/pinpt/test/hooks/1" ||
					req.URL.Path == "/repos/pinpt/test2/hooks/1" ||
					req.URL.Path == "/repos/pinpt/test3/hooks/1")
			rw.WriteHeader(http.StatusNoContent)
			return
		}

		if req.Method == http.MethodPost && req.URL.Path == "/repos/pinpt/test/hooks" {

			bts, err := ioutil.ReadAll(req.Body)
			assert.NoError(err)

			actual := string(bts)
			expected := string(`{"name":"web","active":true,"events":["wh1","wh2"],"config":{"url":"https://test.example.com","content_type":"json","insecure_ssl":"0"}}`)

			assert.Equal(expected, actual)

			rw.Write(bts)

			return
		}

		if req.Method == http.MethodPost && req.URL.Path == "/repos/pinpt/test2/hooks" {

			bts, err := ioutil.ReadAll(req.Body)
			assert.NoError(err)

			actual := string(bts)
			expected := string(`{"name":"web","active":true,"events":["wh1"],"config":{"url":"https://test.example.com","content_type":"json","insecure_ssl":"0"}}`)

			assert.Equal(expected, actual)

			rw.Write(bts)

			return
		}

		if req.Method == http.MethodGet && req.URL.Path == "/repos/pinpt/test3/hooks" {
			wh := webhook{
				ID:     1,
				Events: []string{"wh1"},
				Config: struct {
					URL string "json:\"url\""
				}{URL: "https://test.example2.com"},
				CreatedAt: firstDate,
			}

			wh2 := webhook{
				ID:     2,
				Events: []string{"wh1"},
				Config: struct {
					URL string "json:\"url\""
				}{URL: "https://test.example2.com"},
				CreatedAt: firstDate.Add(time.Second),
			}

			var err error
			bts, err = json.Marshal([]webhook{wh, wh2})
			if err != nil {
				panic(err)
			}
			rw.Write(bts)
			return
		}

		if req.Method == http.MethodGet {
			wh := webhook{
				ID:     1,
				Events: []string{"wh1"},
				Config: struct {
					URL string "json:\"url\""
				}{URL: "https://test.example.com"},
				CreatedAt: firstDate,
			}

			var err error
			bts, err = json.Marshal([]webhook{wh})
			if err != nil {
				panic(err)
			}
		}

		if req.Method == http.MethodGet && req.URL.Path == "/repos/pinpt/noperm/hooks" {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		rw.Write(bts)
	}))
	defer server.Close()

	logger := hclog.New(&hclog.LoggerOptions{
		Name: "test",
	})

	qc := QueryContext{
		APIURL3: server.URL,
		Logger:  logger,
		Clients: reqstats.New(reqstats.Opts{
			Logger:                logger,
			TLSInsecureSkipVerify: true,
		}).Clients,
	}

	// don't do anything
	testRepo := Repo{
		ID:            "1",
		NameWithOwner: "pinpt/test",
	}

	err := WebhookCreateIfNotExists(qc, testRepo, "https://test.example.com", []string{"wh1"}, WebhookReplaceOlderThan)
	assert.NoError(err)

	// no permissions
	noPerm := Repo{
		ID:            "1",
		NameWithOwner: "pinpt/noperm",
	}

	err = WebhookCreateIfNotExists(qc, noPerm, "https://test.example.com", []string{"wh1"}, WebhookReplaceOlderThan)
	assert.Equal(errors.New("no permissions to list webhooks for repo"), err)

	// bad replace date
	err = WebhookCreateIfNotExists(qc, testRepo, "https://test.example.com", []string{"wh1"}, "baddate")
	assert.True(strings.Contains(err.Error(), "invalid webhookReplaceOlderThan constant format"))

	// bad webhook url
	err = WebhookCreateIfNotExists(qc, testRepo, " http://foo.com", []string{"wh1"}, WebhookReplaceOlderThan)
	assert.Error(err)

	// update webhook
	err = WebhookCreateIfNotExists(qc, testRepo, "https://test.example.com", []string{"wh1", "wh2"}, WebhookReplaceOlderThan)
	assert.NoError(err)

	// update with date webhook
	testRepo2 := Repo{
		ID:            "1",
		NameWithOwner: "pinpt/test2",
	}
	err = WebhookCreateIfNotExists(qc, testRepo2, "https://test.example.com", []string{"wh1"}, secondDateStr)
	assert.NoError(err)

	// just delete old pinpt webhooks
	testRepo3 := Repo{
		ID:            "3",
		NameWithOwner: "pinpt/test3",
	}
	err = WebhookCreateIfNotExists(qc, testRepo3, "https://test.example2.com", []string{"wh1"}, secondDateStr)
	assert.NoError(err)

}
