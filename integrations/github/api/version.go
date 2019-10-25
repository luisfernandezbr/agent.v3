package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pinpt/agent.next/pkg/requests"
)

// EnterpriseVersion returns the major version of enterprise install.
// Since there is not endpoint for getting the actual version,
// we rely on the fact that when you make an
// unauthenticated request the api return link to docs with version in the url.
func EnterpriseVersion(qc QueryContext, apiURL string) (version string, rerr error) {
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		rerr = err
		return
	}
	var respJSON struct {
		URL string `json:"documentation_url"`
	}
	reqs := requests.New(qc.Logger, qc.Clients.TLSInsecure)

	resp, logError, err := reqs.Do(context.TODO(), req)
	if err != nil {
		rerr = err
		return
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		rerr = logError(err)
		return
	}
	err = json.Unmarshal(b, &respJSON)
	if err != nil {
		rerr = logError(err)
		return
	}
	return extractMajorVersion(respJSON.URL)
}

func extractMajorVersion(docsURL string) (_ string, rerr error) {
	if docsURL == "" {
		rerr = errors.New("could not get enterprise version documentation_url is empty")
		return
	}
	prefix := "https://developer.github.com/enterprise/"
	suffix := "/v4"
	if !strings.HasPrefix(docsURL, prefix) || !strings.HasSuffix(docsURL, suffix) {
		rerr = fmt.Errorf("unexpected documentation_url: %v", docsURL)
		return
	}
	u := strings.TrimPrefix(docsURL, prefix)
	u = strings.TrimSuffix(u, suffix)
	if len(u) != 4 {
		rerr = fmt.Errorf("unexpected documentation_url, wanted version len=4: %v", docsURL)
		return
	}
	return u, nil
}
