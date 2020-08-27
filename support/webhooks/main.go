package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/pinpt/go-common/v10/api"
	"github.com/pinpt/go-common/v10/event"
	pstrings "github.com/pinpt/go-common/v10/strings"
	"github.com/pinpt/integration-sdk/agent"
)

type createHookRequest struct {
	CustomerID string            `json:"customer_id,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	System     string            `json:"system,omitempty"`
}

const customerID = "e31ab2c866a8cadc"
const uuid = "57f96afd-528c-4bda-ae5b-b4683c971f33"
const integrationID = "2a524dc3fd462e55"

func apiKey() string {
	apiKey := os.Getenv("PP_INTERNAL_API_KEY")
	if apiKey == "" {
		panic("no key")
	}
	return apiKey
}

func register() string {
	u := pstrings.JoinURL(api.BackendURL(api.EventService, "dev"), "hook")
	hookReqPayload := createHookRequest{
		CustomerID: customerID,
		Headers: map[string]string{
			"customer_id": customerID,
			//"uuid":           uuid,
			"integration_id": integrationID,
		},
		System: "agent-incrementals",
	}
	hookReqBytes, err := json.Marshal(hookReqPayload)
	if err != nil {
		panic(err)
	}
	hookReq, err := http.NewRequest("POST", u, bytes.NewReader(hookReqBytes))
	if err != nil {
		panic(err)
	}

	hookReq.Header.Set("Content-Type", "application/json")
	hookReq.Header.Set("x-api-key", apiKey())

	resp, err := http.DefaultClient.Do(hookReq)
	if err != nil {
		panic(err)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	var hookRes map[string]interface{}
	err = json.Unmarshal(b, &hookRes)
	if err != nil {
		panic(err)
	}
	hookURL, _ := hookRes["url"].(string)
	if hookURL == "" {
		panic(fmt.Errorf("empty hookURL %v", string(b)))
	}
	fmt.Println("got hook url", hookURL)
	fmt.Println("sending test hook")
	sendEvent(hookURL, []byte(`{"k":"v"}`))
	return hookURL
}

func proxy() {
	//hookURL := register()

	fmt.Println("starting webhooks server")
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		fmt.Println("X-GitHub-Event", r.Header.Get("X-GitHub-Event"))
		fmt.Println(string(b))

		return

		var data map[string]interface{}
		err = json.Unmarshal(b, &data)
		if err != nil {
			panic(err)
		}
		data["x-github-event"] = r.Header.Get("X-GitHub-Event")

		b, err = json.Marshal(data)
		if err != nil {
			panic(err)
		}

		//sendEvent(hookURL, b)

		w.WriteHeader(200)
	})

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "register" {
		register()
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "proxy" {
		proxy()
		return
	}
}

func sendEvent(hookURL string, data []byte) {
	req, err := http.NewRequest("POST", hookURL, bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}

	fmt.Println("got from event-api", resp.StatusCode)
}

func sendEvent2(_ string, data []byte) {
	headers := map[string]string{
		"customer_id": customerID,
		"uuid":        uuid,
	}
	obj := &agent.IntegrationMutationRequest{}
	obj.Data = string(data)
	//obj := &web.Hook{}
	err := event.Publish(context.Background(), event.PublishEvent{Object: obj, Headers: headers, Logger: nil}, "dev", "", func(config *event.PublishConfig) error {
		config.Header.Add("x-api-key", apiKey())
		return nil
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("sent event 2")
}
