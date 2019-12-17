package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/pinpt/agent/pkg/requests"
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
	version, err = extractMajorVersion(respJSON.URL)
	if err != nil {
		rerr = fmt.Errorf("could not get enterprise version from documentation_url, url: %v err: %v", respJSON.URL, err)
		return
	}
	return
}

func extractMajorVersion(docsURL string) (_ string, rerr error) {
	if docsURL == "" {
		rerr = errors.New("url is empty")
		return
	}
	u, err := url.Parse(docsURL)
	if err != nil {
		rerr = err
		return
	}
	p := u.Path
	p = strings.Trim(p, "/")
	p = strings.TrimPrefix(p, "enterprise/")

	if strings.HasSuffix(p, "/v4") {
		p = strings.TrimSuffix(p, "/v4")
	} else if strings.HasSuffix(p, "/v3") {
		p = strings.TrimSuffix(p, "/v3")
	} else {
		rerr = fmt.Errorf("unexpected url: %v", docsURL)
		return
	}
	if len(p) != 4 {
		rerr = fmt.Errorf("unexpected documentation_url, wanted version len=4, p: %v", p)
		return
	}
	return p, nil
}
