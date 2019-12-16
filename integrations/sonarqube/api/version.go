package api

import (
	"crypto/tls"
	"io/ioutil"
	"net/http"

	"github.com/pinpt/go-common/httpdefaults"
	pstring "github.com/pinpt/go-common/strings"
)

func (a *SonarqubeAPI) APIVersion() (apiVersion string, err error) {

	c := &http.Client{}
	transport := httpdefaults.DefaultTransport()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: false}
	c.Transport = transport

	url := pstring.JoinURL(a.url, "server", "version")

	var req *http.Request
	req, err = http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return
	}

	req.SetBasicAuth(a.authToken, "")

	var resp *http.Response
	resp, err = c.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	bts, _ := ioutil.ReadAll(resp.Body)

	apiVersion = string(bts)

	return
}
