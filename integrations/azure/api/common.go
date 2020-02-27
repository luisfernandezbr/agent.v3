package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	pjson "github.com/pinpt/go-common/json"

	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/ids2"
	"github.com/pinpt/go-common/httpdefaults"

	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/httpclient"
)

const maxResults int = 300

type stringmap map[string]string

// Creds a credentials object, all properties are required
type Creds struct {
	URL            string `json:"url"`
	CollectionName string `json:"collection_name"`
	Organization   string `json:"organization"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	APIKey         string `json:"api_key"`     // https://your_url/tfs/DefaultCollection/_details/security/tokens
	APIVersion     string `json:"api_version"` // https://your_url/tfs/DefaultCollection/_details/security/tokens
}

// API the api object for azure/fts
type API struct {
	creds      *Creds
	customerid string
	reftype    string

	client      *httpclient.HTTPClient
	logger      hclog.Logger
	tfs         bool
	apiversion  string
	concurrency int

	IDs ids2.Gen
}

// NewAPI initializer
func NewAPI(ctx context.Context, logger hclog.Logger, concurrency int, customerid, reftype string, creds *Creds, istfs bool) *API {
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
	api := &API{
		creds:       creds,
		client:      httpclient.NewHTTPClient(ctx, conf, client),
		logger:      logger,
		customerid:  customerid,
		reftype:     reftype,
		tfs:         istfs,
		apiversion:  creds.APIVersion,
		concurrency: concurrency,
		IDs:         ids2.New(customerid, reftype),
	}
	if api.apiversion == "" {
		if istfs {
			api.apiversion = "3.0"
		} else {
			api.apiversion = "5.1"
		}
	}
	return api
}

func (api *API) postRequest(endPoint string, params stringmap, body interface{}, out interface{}) error {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewBuffer(b)
	}
	return api.doRequest(http.MethodPost, endPoint, params, reader, out)
}

func (api *API) GetRequest(endPoint string, params stringmap, out interface{}) error {
	return api.getRequest(endPoint, params, out)
}
func (api *API) getRequest(endPoint string, params stringmap, out interface{}) error {
	if params == nil {
		params = stringmap{}
	}
	if _, o := params["pagingoff"]; !o {
		if _, o := params["$top"]; !o {
			params["$top"] = strconv.Itoa(maxResults)
		}
		if _, o := params["$skip"]; !o {
			params["$skip"] = "0"
		}
	}
	return api.doRequest(http.MethodGet, endPoint, params, nil, out)
}

func (api *API) doRequest(method, endPoint string, params stringmap, reader io.Reader, out interface{}) error {

	var rawurl string
	if api.tfs {
		rawurl = pstrings.JoinURL(api.creds.URL, api.creds.CollectionName, endPoint)
	} else {
		rawurl = pstrings.JoinURL(api.creds.URL, api.creds.Organization, endPoint)
	}
	u, _ := url.Parse(rawurl)
	vals := u.Query()
	if vals.Get("api-version") == "" {
		vals.Add("api-version", api.apiversion)
	}
	for k, v := range params {
		vals.Set(k, v)
	}
	u.RawQuery = vals.Encode()
	req, err := http.NewRequest(method, u.String(), reader)
	if err != nil {
		return err
	}
	req.SetBasicAuth("", api.creds.APIKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := api.client.Do(req)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if method == http.MethodGet {
		b = append([]byte{'['}, b...)
		b = append(b, ']')
	}
	defer res.Body.Close()
	//api.logger.Debug("response data", "b", string(b))
	if res.StatusCode == http.StatusOK {
		err := json.Unmarshal(b, &out)
		if err != nil {
			rerr := err
			var r []interface{}
			err = json.Unmarshal(b, &r)
			if err != nil {
				return fmt.Errorf("invalid json: response code: %v request url: %v %v", res.StatusCode, res.Request.URL, err)
			}
			return fmt.Errorf("invalid json object: response code: %v request url: %v\npayload: %v err: %v", res.StatusCode, res.Request.URL, stringify(r), rerr)
		}
		return nil
	}
	return fmt.Errorf("invalid response code: %v request url: %v", res.StatusCode, res.Request.URL)
}

// some util functions
func exists(what string, where []string) bool {
	if where == nil {
		return false
	}
	for _, each := range where {
		if what == each {
			return true
		}
	}
	return false
}

func stringify(i interface{}) string {
	return pjson.Stringify(i, true)
}

func itemStateName(state string, itemtype string) string {
	if state == "" {
		return ""
	}
	return fmt.Sprintf("%s (%s)", state, itemtype)
}
