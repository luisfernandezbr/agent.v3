package api

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/pinpt/httpclient"
)

type tfsPaginator struct {
}

// make sure it implements the interface
var _ httpclient.Paginator = (*tfsPaginator)(nil)

func (tfsPaginator) HasMore(page int, req *http.Request, resp *http.Response) (bool, *http.Request) {
	var skippaging bool
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
	if err != nil {
		return false, nil
	}
	body, _ := json.Marshal(mapBody.Value)
	// special case, pull requests
	skippaging, body, err = paginatePullRequest(req.URL, body)

	body = bytes.TrimPrefix(body, []byte("["))
	body = bytes.TrimSuffix(body, []byte("]"))
	if page > 1 {
		body = append([]byte{','}, body...)
	}
	resp.Body = ioutil.NopCloser(bytes.NewReader(body))
	if !skippaging && mapBody.Count == int64(maxResults) {
		urlquery := req.URL.Query()
		urlquery.Set("$skip", strconv.Itoa(maxResults*page))
		req.URL.RawPath = urlquery.Encode()
		newreq, _ := http.NewRequest(req.Method, req.URL.String(), nil)
		if user, pass, ok := req.BasicAuth(); ok {
			newreq.SetBasicAuth(user, pass)
		}
		return true, newreq
	}
	return false, nil
}
