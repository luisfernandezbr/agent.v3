package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

func (s *Integration) makeRequest(query string, res interface{}) error {
	data := map[string]string{
		"query": query,
	}

	b, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest("POST", "https://api.github.com/graphql", bytes.NewReader(b))
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
