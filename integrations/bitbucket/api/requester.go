package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/httpdefaults"
	pstrings "github.com/pinpt/go-common/strings"
)

type RequesterOpts struct {
	Logger             hclog.Logger
	APIURL             string
	Username           string
	Password           string
	InsecureSkipVerify bool
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

func (s *Requester) Request(objPath string, params url.Values, paginable bool, res interface{}) (page PageInfo, err error) {

	u := pstrings.JoinURL(s.opts.APIURL, objPath)

	if paginable {
		tags := getJsonTags(res)
		// This parameters will helpus get only the fields we need
		// This reduce the time from ~27s to ~12s
		params.Set("fields", tags)
	}

	if len(params) != 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return
	}
	req.SetBasicAuth(s.opts.Username, s.opts.Password)

	s.logger.Debug("request", "url", req.URL.String())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		s.logger.Info("api request failed", "url", u)
		return page, fmt.Errorf(`bitbucket returned invalid status code: %v`, resp.StatusCode)
	}

	if paginable {
		var response Response

		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return
		}

		if err = json.Unmarshal(response.Values, &res); err != nil {
			return
		}

		u, _ := url.Parse(response.Next)

		page.PageSize = response.PageLen
		page.Page = response.Page
		page.NextPage = u.Query().Get("page")

	} else {
		if err = json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return page, err
		}
	}

	return
}

type Response struct {
	TotalValues int64           `json:"size"`
	Page        int64           `json:"page"`
	PageLen     int64           `json:"pagelen"`
	Next        string          `json:"next"`
	Values      json.RawMessage `json:"values"`
}

func getJsonTags(i interface{}) string {
	typ := reflect.TypeOf(i)
	tags := getJsonTagsFromType(typ)
	tags = appendPrefix("values", tags)
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
