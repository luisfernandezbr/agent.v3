package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/oauthtoken"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/go-common/httpdefaults"
	pstrings "github.com/pinpt/go-common/strings"
)

type RequesterOpts struct {
	Logger             hclog.Logger
	APIURL             string
	Username           string
	Password           string
	UseOAuth           bool
	OAuth              *oauthtoken.Manager
	InsecureSkipVerify bool
	Agent              rpcdef.Agent
}

type internalRequest struct {
	URL      string
	Params   url.Values
	Pageable bool
	Response interface{}
	PageInfo PageInfo
}

func NewRequester(opts RequesterOpts) *Requester {
	s := &Requester{}
	s.opts = opts
	s.logger = opts.Logger

	{
		c := &http.Client{}
		transport := httpdefaults.DefaultTransport()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: opts.InsecureSkipVerify}
		c.Transport = transport
		s.httpClient = c
	}

	return s
}

type Requester struct {
	logger     hclog.Logger
	opts       RequesterOpts
	httpClient *http.Client
}

func (s *Requester) setAuth(req *http.Request) {
	if s.opts.UseOAuth {
		req.Header.Set("Authorization", "Bearer "+s.opts.OAuth.Get())
	} else {
		req.SetBasicAuth(s.opts.Username, s.opts.Password)
	}
}

// Request make request
func (e *Requester) Request(url string, params url.Values, pageable bool, response interface{}) (pi PageInfo, err error) {

	ir := &internalRequest{
		URL:      url,
		Params:   params,
		Pageable: pageable,
		Response: response,
	}

	return e.makeRequestRetry(ir, 0)

}

const maxGeneralRetries = 2

func (e *Requester) makeRequestRetry(req *internalRequest, generalRetry int) (pageInfo PageInfo, err error) {
	var isRetryable bool
	isRetryable, pageInfo, err = e.request(req, generalRetry+1)
	if err != nil {
		if !isRetryable {
			return pageInfo, err
		}
		if generalRetry >= maxGeneralRetries {
			return pageInfo, fmt.Errorf(`can't retry request, too many retries, err: %v`, err)
		}
		return e.makeRequestRetry(req, generalRetry+1)
	}
	return
}

func (e *Requester) request(r *internalRequest, retryThrottled int) (isErrorRetryable bool, pi PageInfo, rerr error) {

	u := pstrings.JoinURL(e.opts.APIURL, r.URL)

	if r.Pageable && r.Params.Get("fields") == "" {
		tags := getJsonTags(r.Response)
		// This parameters will help us get only the fields we need
		// This reduce the time from ~27s to ~12s
		r.Params.Set("fields", tags)
	}

	if len(r.Params) != 0 {
		u += "?" + r.Params.Encode()
	}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		rerr = err
		return
	}
	e.setAuth(req)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		rerr = err
		return
	}
	defer resp.Body.Close()

	e.logger.Debug("api request", "url", u, "status", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {

		if resp.StatusCode == http.StatusUnauthorized {
			if e.opts.UseOAuth {
				if rerr = e.opts.OAuth.Refresh(); rerr != nil {
					return false, pi, rerr
				}
				return true, pi, fmt.Errorf("error refreshing token")
			}
			return false, pi, fmt.Errorf("request not authorized")
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			waitTime := time.Minute // according to docs there is quota available every minute
			paused := time.Now()
			e.opts.Agent.SendPauseEvent(fmt.Sprintf("bitbucket rate limit hit, will resume in %v ", waitTime), paused.Add(waitTime))
			time.Sleep(waitTime)
			e.opts.Agent.SendResumeEvent(fmt.Sprintf("bitbucket resumed, time elapsed %v", time.Since(paused)))
			return true, pi, fmt.Errorf("rate limit hit")
		}

		if resp.StatusCode == http.StatusNotFound {
			e.logger.Warn("the source or destination could not be found", "url", u)
			return false, pi, nil
		}

		e.logger.Debug("api request failed", "url", u, "status", resp.StatusCode)
		return true, pi, fmt.Errorf(`bitbucket returned invalid status code: %v`, resp.StatusCode)
	}

	if r.Pageable {
		var response Response

		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return
		}

		if err = json.Unmarshal(response.Values, &r.Response); err != nil {
			return
		}

		u, _ := url.Parse(response.Next)

		pi.PageSize = response.PageLen
		pi.Page = response.Page
		pi.NextPage = u.Query().Get("page")
		pi.Total = response.TotalValues

	} else {
		if err = json.NewDecoder(resp.Body).Decode(&r.Response); err != nil {
			return false, pi, err
		}
	}

	return
}

type Response struct {
	TotalValues int             `json:"size"`
	Page        int64           `json:"page"`
	PageLen     int64           `json:"pagelen"`
	Next        string          `json:"next"`
	Values      json.RawMessage `json:"values"`
}

func getJsonTags(i interface{}) string {
	typ := reflect.TypeOf(i)
	tags := getJsonTagsFromType(typ)
	tags = appendPrefix("values", tags)
	tags = append(tags, "pagelen")
	tags = append(tags, "page")
	tags = append(tags, "size")
	tags = append(tags, "next")
	joinTags := strings.Join(tags, ",")
	return joinTags
}

func getJsonTagsFromType(typ reflect.Type) (names []string) {
	if typ.Kind() == reflect.Array || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Ptr {
		return getJsonTagsFromType(typ.Elem())
	} else {
		for i, total := 0, typ.NumField(); i < total; i++ {
			fieldType := typ.Field(i)
			if fieldType.Type.Kind() == reflect.Struct && fieldType.Type.Name() != "Time" {
				newValues := getJsonTagsFromType(fieldType.Type)
				prefix := fieldType.Tag.Get("json")
				names = append(names, appendPrefix(prefix, newValues)...)
			} else {
				names = append(names, fieldType.Tag.Get("json"))
			}
		}
	}
	return
}

func appendPrefix(prefix string, names []string) (newnames []string) {
	for _, name := range names {
		newnames = append(newnames, prefix+"."+name)
	}
	return
}
