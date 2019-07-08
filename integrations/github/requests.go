package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

const maxRetries = 3

func (s *Integration) makeRequest(query string, res interface{}) error {
	s.requestConcurrencyChan <- true
	defer func() {
		<-s.requestConcurrencyChan
	}()
	return s.makeRequestRetry(query, res, maxRetries)
}

func (s *Integration) makeRequestRetry(query string, res interface{}, retry int) error {

	data := map[string]string{
		"query": query,
	}

	b, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest("POST", s.config.APIURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "bearer "+s.config.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {

		if resp.StatusCode == 403 && strings.Contains(string(b), "wait") {
			if retry == 0 {
				s.logger.Info("api request failed", "body", string(b))
				return fmt.Errorf(`resp resp.StatusCode != 200, got %v, can't retry, too many retries already`, resp.StatusCode)
			}
			s.logger.Warn("api request failed with status 403 (due to throttling), will sleep for 30m and retry, this should only happen if hourly quota is used up, check here (https://developer.github.com/v4/guides/resource-limitations/#returning-a-calls-rate-limit-status)", "body", string(b), "retry", retry)
			time.Sleep(30 * time.Minute)
			return s.makeRequestRetry(query, res, retry-1)
		}

		s.logger.Info("api request failed", "body", string(b))
		return fmt.Errorf(`resp resp.StatusCode != 200, got %v`, resp.StatusCode)
	}

	// check if there were errors returned first

	var errRes struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	json.Unmarshal(b, &errRes)
	if len(errRes.Errors) != 0 {
		s.logger.Info("api request failed", "body", string(b))
		return errors.New("api request failed: " + errRes.Errors[0].Message)
	}

	//s.logger.Info("response body", string(b))

	err = json.Unmarshal(b, &res)
	if err != nil {
		return err
	}
	return nil
}
