package azureapi

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/httpclient"
)

type tfsPaginator struct {
	logger hclog.Logger
}

// make sure it implements the interface
var _ httpclient.Paginator = (*tfsPaginator)(nil)

func (p tfsPaginator) HasMore(page int, req *http.Request, resp *http.Response) (bool, *http.Request) {
	var err error
	var mapBody struct {
		Count int64         `json:"count"`
		Value []interface{} `json:"value"`
	}
	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, nil
	}
	err = json.Unmarshal(raw, &mapBody)
	if err != nil || mapBody.Value == nil {
		resp.Body = ioutil.NopCloser(bytes.NewReader(raw))
		return false, nil
	}
	body, _ := json.Marshal(mapBody.Value)
	body = bytes.TrimPrefix(body, []byte("["))
	body = bytes.TrimSuffix(body, []byte("]"))
	if page > 1 {
		body = append([]byte{','}, body...)
	}
	resp.Body = ioutil.NopCloser(bytes.NewReader(body))
	if mapBody.Count == int64(maxResults) {
		urlquery := req.URL.Query()
		if urlquery.Get("pagingoff") != "" {
			return false, nil
		}
		top := maxResults
		if t := urlquery.Get("$top"); t != "" {
			top, _ = strconv.Atoi(t)
		}
		urlquery.Set("$skip", strconv.Itoa(top*page))
		req.URL.RawQuery = urlquery.Encode()
		newreq, _ := http.NewRequest(req.Method, req.URL.String(), nil)
		if user, pass, ok := req.BasicAuth(); ok {
			newreq.SetBasicAuth(user, pass)
		}
		return true, newreq
	}
	return false, nil
}
