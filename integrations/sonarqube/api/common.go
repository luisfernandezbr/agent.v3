package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pinpt/go-common/httpdefaults"
	pstring "github.com/pinpt/go-common/strings"
	"github.com/pinpt/httpclient"
)

// Project is the project detail
type Project struct {
	ID           string `json:"id"`
	Key          string `json:"key"`
	Name         string `json:"name"`
	Organization string `json:"organization"`
	Qualifier    string `json:"qualifier"`
	Project      string `json:"project"`
}

// RawMetric is the Metric detail
type RawMetric struct {
	Date  string `json:"date"`
	Value string `json:"value"`
}

// Measure is the measure detail
type Measure struct {
	Metric  string       `json:"metric"`
	History []*RawMetric `json:"history"`
}

// Metric full metric object composed with project data
type Metric struct {
	ProjectID  string    `json:"projectId"`
	ProjectKey string    `json:"projectKey"`
	ID         string    `json:"id"`
	Date       time.Time `json:"date"`
	Metric     string    `json:"metric"`
	Value      string    `json:"value"`
}

// SonarqubeAPI ...
type SonarqubeAPI struct {
	url       string
	authToken string
	metrics   []string
	client    *httpclient.HTTPClient
}

func NewSonarqubeAPI(ctx context.Context, url string, authToken string, metrics []string) *SonarqubeAPI {

	transport := httpdefaults.DefaultTransport()
	if !strings.Contains(url, "sonarcloud.io") {
		// if a self-service installation allow self-signed certificates
		// TODO: make this configurable
		transport.TLSClientConfig = &tls.Config{}
		transport.TLSClientConfig.InsecureSkipVerify = true
	}
	hcConfig := &httpclient.Config{
		Paginator: httpclient.InBodyPaginator(),
		Retryable: httpclient.NewBackoffRetry(10*time.Millisecond, 100*time.Millisecond, 60*time.Second, 2.0),
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Minute,
	}
	a := &SonarqubeAPI{
		url:       url,
		authToken: authToken,
		metrics:   metrics,
		client:    httpclient.NewHTTPClient(ctx, hcConfig, client),
	}
	return a
}

// Validate ...
func (a *SonarqubeAPI) Validate() (bool, error) {

	var val struct {
		Valid bool `json:"valid"`
	}
	err := a.doRequest("GET", "/authentication/validate", time.Time{}, &val)
	if err != nil {
		return false, err
	}
	return val.Valid, nil

}

func (a *SonarqubeAPI) doRequest(method string, endPoint string, fromDate time.Time, obj interface{}) error {
	if a.url == "" {
		return fmt.Errorf("Sonarqube API missing `url` property")
	}
	if a.authToken == "" {
		return fmt.Errorf("Sonarqube API missing `authToken` property")
	}
	if len(a.metrics) == 0 {
		return fmt.Errorf("Sonarqube API missing `metrics` property")
	}
	url := pstring.JoinURL(a.url, endPoint)

	addFrom := func(url string, from time.Time) string {
		str := from.Format("2006-01-02T15:04:05-0700")
		// There seems to be a bug in Sonarqube api where it fails if the from date's
		// time zone is -0 instead of +0
		str = strings.Replace(str, "+0", "-0", 1)
		if strings.Contains(url, "?") {
			return url + "&from=" + str
		}
		return url + "?from=" + str
	}
	if !fromDate.IsZero() {
		url = addFrom(url, fromDate)
	}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(a.authToken, "")

	res, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&obj); err != nil {
		return err
	}
	return nil
}
