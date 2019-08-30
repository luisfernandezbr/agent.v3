package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	purl "net/url"
	"regexp"

	"github.com/pinpt/httpclient"
)

type tfsPaginator struct {
}

// make sure it implements the interface
var _ httpclient.Paginator = (*tfsPaginator)(nil)

func (tfsPaginator) HasMore(page int, req *http.Request, resp *http.Response) (bool, *http.Request) {

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
	body = bytes.TrimPrefix(body, []byte("["))
	body = bytes.TrimSuffix(body, []byte("]"))
	if page > 1 {
		body = append([]byte{','}, body...)
	}
	resp.Body = ioutil.NopCloser(bytes.NewReader(body))
	if mapBody.Count == int64(maxResults) {
		url := req.URL.String()
		skipreg := regexp.MustCompile(fmt.Sprintf(`([\&|\?]\%s)([\d]+)`, purl.QueryEscape(`$skip`)+`=`))
		url = skipreg.ReplaceAllString(url, "")
		jobj := params{
			"$skip": maxResults * page,
		}
		url = urlWithParams(url, jobj)
		newreq, _ := http.NewRequest(req.Method, url, nil)
		if user, pass, ok := req.BasicAuth(); ok {
			newreq.SetBasicAuth(user, pass)
		}
		return true, newreq
	}
	return false, nil
}
