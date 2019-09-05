package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/ids"
	"github.com/pinpt/go-common/httpdefaults"
	pstring "github.com/pinpt/go-common/strings"
	"github.com/pinpt/httpclient"
)

const maxResults int = 300
const apiVersion string = "3.0"

type params map[string]string

// Creds a credentials object, all properties are required
type Creds struct {
	URL        string `json:"url"`
	Collection string `json:"collection"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	APIKey     string `json:"apitoken"` // https://your_url/tfs/DefaultCollection/_details/security/tokens
}

// TFSAPI the api object for fts
type TFSAPI struct {
	creds      *Creds
	customerid string
	reftype    string

	client *httpclient.HTTPClient
	logger hclog.Logger
}

func (s *TFSAPI) RepoID(refid string) string {
	return ids.CodeRepo(s.customerid, s.reftype, refid)
}

func (s *TFSAPI) UserID(refid string) string {
	return ids.CodeUser(s.customerid, s.reftype, refid)
}

func (s *TFSAPI) PullRequestID(repoid, refid string) string {
	return ids.CodePullRequest(s.customerid, s.reftype, repoid, refid)
}

func (s *TFSAPI) BranchID(repoid, branchname, firstsha string) string {
	return ids.CodeBranch(s.customerid, s.reftype, repoid, branchname, firstsha)
}

// NewTFSAPI initializer
func NewTFSAPI(ctx context.Context, logger hclog.Logger, customerid, reftype string, creds *Creds) *TFSAPI {
	client := &http.Client{
		Transport: httpdefaults.DefaultTransport(),
		Timeout:   10 * time.Minute,
	}
	conf := &httpclient.Config{
		Paginator: tfsPaginator{
			logger: logger,
		},
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

	rawurl := pstring.JoinURL(a.creds.URL, a.creds.Collection, endPoint)
	var reader io.Reader
	if jobj == nil {
		jobj = params{}
	}
	u, _ := url.Parse(rawurl)
	vals := u.Query()
	vals.Add("$top", strconv.Itoa(maxResults))
	vals.Add("$skip", "0")
	vals.Add("api-version", apiVersion)
	if !fromdate.IsZero() {
		vals.Add("searchCriteria.fromDate", fromdate.Format(time.RFC3339))
	}
	for k, v := range jobj {
		vals.Add(k, v)
	}
	u.RawQuery = vals.Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), reader)
	if err != nil {
		panic(err)
	}
	req.SetBasicAuth("", a.creds.APIKey)
	req.Header.Set("Content-Type", "application/json")
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

func exists(what string, array []string) bool {
	for _, each := range array {
		if what == each {
			return true
		}
	}
	return false
}
