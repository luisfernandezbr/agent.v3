package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	purl "net/url"
	"strings"

	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/httpdefaults"
	pstring "github.com/pinpt/go-common/strings"
	"github.com/pinpt/httpclient"
)

const maxResults int = 300
const apiVersion string = "3.0"

type params map[string]interface{}

// Creds a credentials object, all properties are required
type Creds struct {
	URL        string `json:"url"`
	Collection string `json:"collection"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	APIKey     string `json:"api_key"` // https://your_url/tfs/DefaultCollection/_details/security/tokens
}

// TFSAPI the api object for fts
type TFSAPI struct {
	creds      *Creds
	customerid string
	reftype    string

	client *httpclient.HTTPClient
	logger hclog.Logger

	// must implement these to match the ids in the datamodel
	RepoID        func(ref string) string
	UserID        func(ref string) string
	PullRequestID func(ref string) string
	BranchID      func(repoRef, branchName string) string
}

// NewTFSAPI initializer
func NewTFSAPI(ctx context.Context, logger hclog.Logger, customerid, reftype string, creds *Creds) *TFSAPI {
	client := &http.Client{
		Transport: httpdefaults.DefaultTransport(),
		Timeout:   10 * time.Minute,
	}
	conf := &httpclient.Config{
		Paginator: tfsPaginator{},
		Retryable: httpclient.NewBackoffRetry(10*time.Millisecond, 100*time.Millisecond, 60*time.Second, 2.0),
	}
	return &TFSAPI{
		creds:      creds,
		client:     httpclient.NewHTTPClient(ctx, conf, client),
		logger:     logger,
		customerid: customerid,
		reftype:    reftype,
	}
}

func (a *TFSAPI) doRequest(endPoint string, jobj params, fromdate time.Time, out interface{}) error {

	url := pstring.JoinURL(a.creds.URL, a.creds.Collection, endPoint)
	var reader io.Reader
	if jobj == nil {
		jobj = params{}
	}
	jobj["$top"] = maxResults
	jobj["$skip"] = 0
	jobj["api-version"] = apiVersion
	if !fromdate.IsZero() {
		jobj["searchCriteria.fromDate"] = fromdate.Format(time.RFC3339)
	}

	url = urlWithParams(url, jobj)
	req, err := http.NewRequest(http.MethodGet, url, reader)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	encToken := base64.StdEncoding.EncodeToString([]byte(":" + a.creds.APIKey))
	req.Header.Set("Authorization", "Basic "+encToken)
	res, err := a.client.Do(req)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	b = append([]byte{'['}, b...)
	b = append(b, ']')
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		return json.Unmarshal(b, &out)
	}
	return fmt.Errorf("response code: %v request url: %v", res.StatusCode, res.Request.URL)
}

func urlWithParams(url string, jobj params) string {
	var parts []string
	for k, v := range jobj {
		var str string
		var ok bool
		str, ok = v.(string)
		if !ok {
			b, _ := json.Marshal(v)
			str = string(b)
		}
		parts = append(parts, purl.QueryEscape(k)+"="+purl.QueryEscape(str))
	}
	if strings.Contains(url, "?") {
		url += "&"
	} else {
		url += "?"
	}
	url += strings.Join(parts, "&")
	return url
}

func exists(what string, array []string) bool {
	for _, each := range array {
		if what == each {
			return true
		}
	}
	return false
}
