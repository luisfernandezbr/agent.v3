package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pinpt/agent/pkg/requests"
)

const checkRateLimitEveryNRequest = 100

func (s *Integration) makeRequest(query string, vars map[string]interface{}, res interface{}) error {
	v := atomic.AddInt64(s.requestsMadeAtomic, 1)
	if v%checkRateLimitEveryNRequest == 0 {
		err := s.checkRateLimitAndSleepIfNecessary()
		if err != nil {
			s.logger.Warn("could not check available rate limit quota, issuing request as normal", "err", err)
		}
	}

	data := map[string]interface{}{
		"query":     query,
		"variables": vars,
	}

	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("could not make request, marshaling of request data failed, err: %v", err)
	}

	u := s.config.APIURL

	req := request{Method: "POST", URL: u, Body: b}

	return s.makeRequestThrottled(req, res)
}

func (s *Integration) makeRequestNoRetries(query string, vars map[string]interface{}, res interface{}) error {
	data := map[string]interface{}{
		"query":     query,
		"variables": vars,
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}
	req := requests.Request{}
	req.Method = "POST"
	req.URL = s.config.APIURL
	req.Body = dataJSON
	req.Header = http.Header{}
	req.Header.Set("Authorization", "bearer "+s.config.Token)
	s.setAcceptHeader(&req.Header)
	req.Header.Set("Content-Type", "application/json")

	resp, err := requests.New(s.logger, s.clients.TLSInsecure).Do(context.Background(), req)
	if err != nil {
		return err
	}
	err = requests.AssertStatusCode(resp.Resp.StatusCode, 200, 299)
	if err != nil {
		return resp.ErrorContext(err)
	}
	b, err := ioutil.ReadAll(resp.Resp.Body)
	if err != nil {
		return resp.ErrorContext(err)
	}

	var errRes struct {
		Errors []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	json.Unmarshal(b, &errRes)
	if len(errRes.Errors) != 0 {
		err1 := errRes.Errors[0]
		err := fmt.Errorf("api request failed: type: %v message %v", err1.Type, err1.Message)
		return resp.ErrorContext(err)
	}

	err = json.Unmarshal(b, &res)
	if err != nil {
		return resp.ErrorContext(err)
	}
	return nil
}

type request struct {
	URL    string
	Method string
	Body   []byte
}

const exportRequestBuffer = 0.2
const minWaitTime = 5 * time.Minute
const maxWaitTime = 30 * time.Minute

func (s *Integration) checkRateLimitAndSleepIfNecessary() error {
	if s.requestsBuffer == 0 {
		return nil
	}

	s.logger.Info("making request to check rate limit quota")

	query := `
	query {
		rateLimit {
			limit
			remaining
			resetAt
		}
	}
	`

	var res struct {
		Data struct {
			RateLimit struct {
				Limit     int       `json:"limit"`
				Remaining int       `json:"remaining"`
				ResetAt   time.Time `json:"resetAt"`
			} `json:"rateLimit"`
		} `json:"data"`
	}

	data := map[string]string{
		"query": query,
	}

	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("could not make request, marshaling of request data failed, err: %v", err)
	}

	u := s.config.APIURL
	req := request{Method: "POST", URL: u, Body: b}

	_, err = s.makeRequestRetryThrottled(req, &res, maxThrottledRetries)
	if err != nil {
		return err
	}

	rl := res.Data.RateLimit
	if rl.Limit == 0 {
		// {"data":{"rateLimit":null}}
		return fmt.Errorf("rateLimit returned invalid object, resulting data Limit is 0")
	}

	if float64(rl.Remaining)/float64(rl.Limit) > s.requestsBuffer {
		// still more than buffer requests left in quota
		return nil
	}

	s.logger.Warn("pausing due to used up request quota, keeping some buffer unused", "remaining", rl.Remaining, "limit", rl.Limit, "wanted_buffer", s.requestsBuffer, "reset_at", rl.ResetAt)

	// used all up-to buffer, pause
	waitTime := rl.ResetAt.Sub(time.Now())
	s.pause(waitTime)
	return nil
}

func (s *Integration) pause(waitTime time.Duration) {
	if waitTime < minWaitTime {
		waitTime = minWaitTime
	}
	if waitTime > maxWaitTime {
		waitTime = maxWaitTime
	}
	paused := time.Now()
	resumeDate := paused.Add(waitTime)

	err := s.agent.SendPauseEvent("", resumeDate)
	if err != nil {
		s.logger.Error("could not send pause event", "err", err)
	}

	time.Sleep(waitTime)

	err = s.agent.SendResumeEvent("")
	if err != nil {
		s.logger.Error("could not resume event", "err", err)
	}
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

func (s *Integration) setAcceptHeader(header *http.Header) {
	// Setting preview header to support github enterprise 2.16
	// pullrequest.timelineItems were a preview feature, and need custom accept header to enable
	// https://developer.github.com/enterprise/2.16/v4/object/pullrequest/
	// https://developer.github.com/enterprise/2.16/v4/previews/#issues-preview
	header.Set("Accept", "application/vnd.github.starfire-preview+json")

	// https://docs.github.com/en/enterprise/2.17/user/graphql/overview/schema-previews#draft-pull-requests-preview
	// https://docs.github.com/en/enterprise/2.18/user/graphql/overview/schema-previews#draft-pull-requests-preview
	// https://docs.github.com/en/enterprise/2.19/user/graphql/overview/schema-previews#draft-pull-requests-preview
	// https://docs.github.com/en/enterprise/2.20/user/graphql/overview/schema-previews#draft-pull-requests-preview
	if strings.Index(s.enterpriseVersion, "2.17") == 0 ||
		strings.Index(s.enterpriseVersion, "2.18") == 0 ||
		strings.Index(s.enterpriseVersion, "2.19") == 0 ||
		strings.Index(s.enterpriseVersion, "2.20") == 0 {
		header.Set("Accept", "application/vnd.github.shadow-cat-preview+json")
	}
}

func (s *Integration) makeRequestRetryThrottled(reqDef request, res interface{}, retryThrottled int) (isErrorRetryable bool, rerr error) {

	req, err := http.NewRequest(reqDef.Method, reqDef.URL, bytes.NewReader(reqDef.Body))
	if err != nil {
		rerr = err
		return
	}

	if bytes.Contains(reqDef.Body, []byte("draft: isDraft")) {
		s.setAcceptHeader(&req.Header)
	}

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
			rerr = fmt.Errorf(`can't retry, too many retries already (resp.StatusCode=%v)`, resp.StatusCode)
			return
		}
		limitReset := resp.Header.Get("X-RateLimit-Reset")
		waitTime := 30 * time.Minute

		// set defaults in case limitReset returns ""
		if limitReset != "" {
			if i, err := strconv.Atoi(limitReset); err != nil {
				s.logger.Error("can't convert X-RateLimit-Reset to number", "err", err)
			} else {
				waitTime = time.Until(time.Unix(int64(i), 0))
			}
		}

		s.logger.Warn("api request failed due to throttling, will sleep and retry, this should only happen if hourly quota is used up, check here (https://developer.github.com/v4/guides/resource-limitations/#returning-a-calls-rate-limit-status)", "body", string(b), "retryThrottled", retryThrottled)

		s.pause(waitTime)

		return s.makeRequestRetryThrottled(reqDef, res, retryThrottled+1)

	}

	// check if there were errors returned first

	if resp.StatusCode != 200 {

		if resp.StatusCode == 403 && strings.Contains(string(b), "You have triggered an abuse detection mechanism. Please wait a few minutes before you try again.") {
			s.logger.Warn("api request failed due to temporary throttling or concurrency being too high, pausing for 5m", "body", string(b), "retryThrottled", retryThrottled)
			s.pause(5 * time.Minute)
			return s.makeRequestRetryThrottled(reqDef, res, retryThrottled+1)
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

		if resp.StatusCode == 502 {
			isErrorRetryable = true
			return
		}
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
		} else if err1.Type == "SERVICE_UNAVAILABLE" {
			s.logger.Warn("service unavailable err", "err", errRes.Errors[0])
			return false, fmt.Errorf("service unavailable err %s", err)
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
