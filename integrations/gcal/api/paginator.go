package api

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/pinpt/httpclient"
)

type paginator struct{}

// make sure it implements the interface
var _ httpclient.Paginator = (*paginator)(nil)

func (p paginator) HasMore(page int, req *http.Request, resp *http.Response) (bool, *http.Request) {

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, nil
	}
	// remove all new lines by unmarshal and marshal
	var res interface{}
	json.Unmarshal(raw, &res)
	raw, _ = json.Marshal(res)

	var paginatorBody struct {
		NextPageToken string `json:"nextPageToken"`
	}
	err = json.Unmarshal(raw, &paginatorBody)
	// if next page token is empty, we're done
	if err != nil || paginatorBody.NextPageToken == "" {
		resp.Body = ioutil.NopCloser(bytes.NewReader(raw))
		return false, nil
	}
	// if we are not done, add a new line to the response
	raw = append(raw, byte('\n'))
	resp.Body = ioutil.NopCloser(bytes.NewReader(raw))

	// set the new page token
	urlquery := req.URL.Query()
	urlquery.Set("pageToken", paginatorBody.NextPageToken)
	req.URL.RawQuery = urlquery.Encode()

	newreq, _ := http.NewRequest(req.Method, req.URL.String(), nil)
	for k, v := range req.Header {
		newreq.Header[k] = v
	}
	return true, newreq
}
