package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

func (s *Integration) makeRequest(query string, res interface{}) error {
	data := map[string]string{
		"query": query,
	}

	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("could not make request, marshaling of request data failed, err: %v", err)
	}

	u := s.config.APIURL

	req := request{Method: "POST", URL: u, Body: b}

	return s.makeRequestThrottled(req, res)
}

type request struct {
	URL    string
	Method string
	Body   []byte
}

func (s *Integration) makeRequestThrottled(req request, res interface{}) error {
	s.requestConcurrencyChan <- true
	defer func() {
		<-s.requestConcurrencyChan
	}()
	return s.makeRequestRetry(req, res, 0)
}

const maxGeneralRetries = 2

func (s *Integration) makeRequestRetry(req request, res interface{}, generalRetry int) error {
	isRetryable, err := s.makeRequestRetryThrottled(req, res, 0)
	if err != nil {
		if !isRetryable {
			return err
		}
		if generalRetry >= maxGeneralRetries {
			return fmt.Errorf(`can't retry request too many retries, err: %v`, err)
		}
		time.Sleep(time.Duration(1+generalRetry) * time.Minute)
		return s.makeRequestRetry(req, res, generalRetry+1)
	}
	return nil
}

const maxThrottledRetries = 3

func (s *Integration) makeRequestRetryThrottled(reqDef request, res interface{}, retryThrottled int) (isErrorRetryable bool, rerr error) {

	req, err := http.NewRequest(reqDef.Method, reqDef.URL, bytes.NewReader(reqDef.Body))
	if err != nil {
		rerr = err
		return
	}

	// Setting preview header to support github enterprise 2.16
	// pullrequest.timelineItems were a preview feature, and need custom accept header to enable
	// https://developer.github.com/enterprise/2.16/v4/object/pullrequest/
	// https://developer.github.com/enterprise/2.16/v4/previews/#issues-preview
	req.Header.Set("Accept", "application/vnd.github.starfire-preview+json")

	req.Header.Set("Authorization", "bearer "+s.config.Token)
	resp, err := s.clients.TLSInsecure.Do(req)
	if err != nil {
		rerr = err
		isErrorRetryable = true
		return
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		rerr = err
		isErrorRetryable = true
		return
	}

	rateLimited := func() (isErrorRetryable bool, rerr error) {
		if retryThrottled >= maxThrottledRetries {
			s.logger.Info("api request failed", "body", string(b))
			rerr = fmt.Errorf(`resp resp.StatusCode != 200, got %v, can't retry, too many retries already`, resp.StatusCode)
			return
		}

		s.logger.Warn("api request failed due to throttling, will sleep for 30m and retry, this should only happen if hourly quota is used up, check here (https://developer.github.com/v4/guides/resource-limitations/#returning-a-calls-rate-limit-status)", "body", string(b), "retryThrottled", retryThrottled)
		time.Sleep(30 * time.Minute)

		return s.makeRequestRetryThrottled(reqDef, res, retryThrottled+1)

	}

	// check if there were errors returned first

	if resp.StatusCode != 200 {

		if resp.StatusCode == 403 && strings.Contains(string(b), "wait") {
			return rateLimited()
		}

		var errRes struct {
			Message string `json:"message"`
		}

		if resp.StatusCode == 401 {
			json.Unmarshal(b, &errRes)
			if errRes.Message != "" {
				rerr = fmt.Errorf(`github request failed with status code %v: %v`, resp.StatusCode, errRes.Message)
				isErrorRetryable = false
				return
			}
		}

		s.logger.Info("api request failed", "body", string(b), "code", resp.StatusCode, "url", s.config.APIURL, "reqBody", string(reqDef.Body))
		rerr = fmt.Errorf(`github request failed with status code %v`, resp.StatusCode)
		isErrorRetryable = false
		return
	}

	var errRes struct {
		Errors []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"errors"`
	}

	json.Unmarshal(b, &errRes)
	if len(errRes.Errors) != 0 {
		s.logger.Info("api request got errors returned in json", "body", string(b))

		err1 := errRes.Errors[0]
		// "{"errors":[{"type":"RATE_LIMITED","message":"API rate limit exceeded"}]}"
		if err1.Type == "RATE_LIMITED" {
			return rateLimited()
		}

		rerr = fmt.Errorf("api request failed: type: %v message %v", err1.Type, err1.Message)
		return
	}

	//s.logger.Info("response body", string(b))

	err = json.Unmarshal(b, &res)
	if err != nil {
		rerr = err
		return
	}

	return
}
