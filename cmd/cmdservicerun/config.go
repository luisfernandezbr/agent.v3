package cmdservicerun

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/pinpt/agent.next/pkg/structmarshal"
)

func convertConfig(in string, c1 map[string]interface{}, exclusions []string) (res map[string]interface{}, integrationName string, rerr error) {
	errStr := func(err string) {
		rerr = errors.New(err)
		return
	}

	res = map[string]interface{}{}
	integrationName = in

	if in == "github" {
		var config struct {
			URL           string   `json:"url"`
			APIToken      string   `json:"apitoken"`
			Organization  string   `json:"organization"`
			ExcludedRepos []string `json:"excluded_repos"`
		}
		err := structmarshal.MapToStruct(c1, &config)
		if err != nil {
			rerr = err
			return
		}
		config.Organization = "pinpt"
		{
			v, ok := c1["api_token"].(string)
			if !ok {
				errStr("missing api_token")
				return
			}
			config.APIToken = v
		}
		{
			v, ok := c1["url"].(string)
			if !ok {
				errStr("missing url")
				return
			}
			config.URL = v
		}
		config.ExcludedRepos = exclusions
		res, err = structmarshal.StructToMap(config)

		if err != nil {
			rerr = err
			return
		}

		return

	} else if in == "jira" {
		var config struct {
			URL              string   `json:"url"`
			Username         string   `json:"username"`
			Password         string   `json:"password"`
			ExcludedProjects []string `json:"excluded_projects"`
		}
		err := structmarshal.MapToStruct(c1, &config)
		if err != nil {
			panic(err)
		}
		us, ok := c1["url"].(string)
		if !ok {
			panic("missing jira url in config")
		}
		u, err := url.Parse(us)
		if err != nil {
			panic(fmt.Errorf("invlid jira url: %v", err))
		}
		integrationName = "jira-hosted"
		if strings.HasSuffix(u.Host, ".atlassian.net") {
			integrationName = "jira-cloud"
		}
		config.URL = us
		{
			v, ok := c1["username"].(string)
			if !ok {
				errStr("missing username")
				return
			}
			config.Username = v
		}
		{
			v, ok := c1["password"].(string)
			if !ok {
				errStr("missing password")
				return
			}
			config.Password = v
		}
		config.ExcludedProjects = exclusions
		res, err = structmarshal.StructToMap(config)
		if err != nil {
			panic(err)
		}

		return
	}

	errStr("missing api_token")
	return
}
