package cmdservicerun

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/pinpt/agent.next/cmd/cmdintegration"

	"github.com/pinpt/agent.next/pkg/encrypt"
	"github.com/pinpt/agent.next/pkg/structmarshal"
)

func configFromEvent(data map[string]interface{}, encryptionKey string) (res cmdintegration.Integration, rerr error) {
	var obj struct {
		Name          string `json:"name"`
		Authorization struct {
			Authorization string `json:"authorization"`
		} `json:"authorization"`
		Exclusions []string `json:"exclusions"`
	}
	err := structmarshal.MapToStruct(data, &obj)
	if err != nil {
		rerr = err
		return
	}

	authEncr := obj.Authorization.Authorization
	if authEncr == "" {
		rerr = errors.New("missing encrypted auth data")
		return
	}

	auth, err := encrypt.DecryptString(authEncr, encryptionKey)
	if err != nil {
		rerr = err
		return
	}

	var authObj map[string]interface{}
	err = json.Unmarshal([]byte(auth), &authObj)
	if err != nil {
		rerr = err
		return
	}

	res.Config, res.Name, err = convertConfig(obj.Name, authObj, obj.Exclusions)
	if err != nil {
		rerr = fmt.Errorf("config object in event is not valid: %v", err)
		return
	}

	return
}

func convertConfig(integrationNameBackend string, configBackend map[string]interface{}, exclusions []string) (configAgent map[string]interface{}, integrationNameAgent string, rerr error) {

	switch integrationNameBackend {

	case "github":
		configAgent, integrationNameAgent, rerr = convertConfigGithub(integrationNameBackend, configBackend, exclusions)
	case "bitbucket":
		configAgent, integrationNameAgent, rerr = convertConfigBitbucket(integrationNameBackend, configBackend, exclusions)
	case "gitlab":
		configAgent, integrationNameAgent, rerr = convertConfigGitlab(integrationNameBackend, configBackend, exclusions)
	case "jira":
		configAgent, integrationNameAgent, rerr = convertConfigJira(integrationNameBackend, configBackend, exclusions)
	case "sonarqube":
		configAgent, integrationNameAgent, rerr = convertConfigSonarqube(integrationNameBackend, configBackend, exclusions)
	case "tfs":
		configAgent, integrationNameAgent, rerr = convertConfigFTS(integrationNameBackend, configBackend, exclusions)
	default:
		rerr = fmt.Errorf("unsupported integration: %v", integrationNameBackend)
		return
	}

	if integrationNameAgent == "" {
		integrationNameAgent = integrationNameBackend
	}

	return
}

func convertConfigGithub(inameBackend string, cb map[string]interface{}, exclusions []string) (res map[string]interface{}, inameAgent string, rerr error) {

	errStr := func(err string) {
		rerr = errors.New(err)
		return
	}

	res = map[string]interface{}{}

	var config struct {
		URL           string   `json:"url"`
		APIToken      string   `json:"apitoken"`
		ExcludedRepos []string `json:"excluded_repos"`
	}

	err := structmarshal.MapToStruct(cb, &config)
	if err != nil {
		rerr = err
		return
	}

	accessToken, _ := cb["access_token"].(string)

	if accessToken != "" {
		// this is github.com cloud auth
		config.APIToken = accessToken
		config.URL = "https://github.com"
	} else {
		{
			v, ok := cb["api_token"].(string)
			if !ok {
				errStr("missing api_token")
				return
			}
			config.APIToken = v
		}
		{
			v, ok := cb["url"].(string)
			if !ok {
				errStr("missing url")
				return
			}
			config.URL = v
		}
	}

	config.ExcludedRepos = exclusions
	res, err = structmarshal.StructToMap(config)

	if err != nil {
		rerr = err
		return
	}

	return
}

func convertConfigGitlab(inameBackend string, cb map[string]interface{}, exclusions []string) (res map[string]interface{}, inameAgent string, rerr error) {

	errStr := func(err string) {
		rerr = errors.New(err)
		return
	}

	res = map[string]interface{}{}

	var config struct {
		URL           string   `json:"url"`
		APIToken      string   `json:"apitoken"`
		ExcludedRepos []string `json:"excluded_repos"`
	}

	err := structmarshal.MapToStruct(cb, &config)
	if err != nil {
		rerr = err
		return
	}

	{
		v, ok := cb["api_token"].(string)
		if !ok {
			errStr("missing api_token")
			return
		}
		config.APIToken = v
	}

	{
		v, ok := cb["url"].(string)
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
}

func convertConfigBitbucket(inameBackend string, cb map[string]interface{}, exclusions []string) (res map[string]interface{}, inameAgent string, rerr error) {

	errStr := func(err string) {
		rerr = errors.New(err)
		return
	}

	res = map[string]interface{}{}

	var config struct {
		URL           string   `json:"url"`
		Username      string   `json:"username"`
		Password      string   `json:"password"`
		ExcludedRepos []string `json:"excluded_repos"`
	}

	err := structmarshal.MapToStruct(cb, &config)
	if err != nil {
		rerr = err
		return
	}

	{
		v, ok := cb["username"].(string)
		if !ok {
			errStr("missing username")
			return
		}
		config.Username = v
	}

	{
		v, ok := cb["password"].(string)
		if !ok {
			errStr("missing password")
			return
		}
		config.Password = v
	}

	{
		v, ok := cb["url"].(string)
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
}

func convertConfigJira(inameBackend string, cb map[string]interface{}, exclusions []string) (res map[string]interface{}, inameAgent string, rerr error) {
	errStr := func(err string) {
		rerr = errors.New(err)
		return
	}

	res = map[string]interface{}{}

	var config struct {
		URL              string   `json:"url"`
		Username         string   `json:"username"`
		Password         string   `json:"password"`
		ExcludedProjects []string `json:"excluded_projects"`

		OauthRefreshToken string `json:"oauth_refresh_token"`
	}
	err := structmarshal.MapToStruct(cb, &config)
	if err != nil {
		panic(err)
	}
	us, ok := cb["url"].(string)
	if !ok {
		errStr("missing jira url in config")
	}
	u, err := url.Parse(us)
	if err != nil {
		rerr = fmt.Errorf("invalid jira url: %v", err)
		return
	}
	inameAgent = "jira-hosted"
	if strings.HasSuffix(u.Host, ".atlassian.net") {
		inameAgent = "jira-cloud"
	}
	config.URL = us

	refreshToken, _ := cb["refresh_token"].(string)
	if refreshToken != "" {
		config.OauthRefreshToken = refreshToken
	} else {
		{
			v, ok := cb["username"].(string)
			if !ok {
				errStr("missing username")
				return
			}
			config.Username = v
		}
		{
			v, ok := cb["password"].(string)
			if !ok {
				errStr("missing password")
				return
			}
			config.Password = v
		}
	}
	config.ExcludedProjects = exclusions
	res, err = structmarshal.StructToMap(config)
	if err != nil {
		rerr = err
		return
	}

	return
}

func convertConfigSonarqube(inameBackend string, cb map[string]interface{}, exclusions []string) (res map[string]interface{}, inameAgent string, rerr error) {
	errStr := func(err string) {
		rerr = errors.New(err)
		return
	}

	res = map[string]interface{}{}

	var config struct {
		URL      string `json:"url"`
		APIToken string `json:"apitoken"`
	}

	err := structmarshal.MapToStruct(cb, &config)
	if err != nil {
		rerr = err
		return
	}

	{
		v, ok := cb["api_token"].(string)
		if !ok {
			errStr("missing api_token")
			return
		}
		config.APIToken = v
	}

	{
		v, ok := cb["url"].(string)
		if !ok {
			errStr("missing url")
			return
		}
		config.URL = v
	}

	res, err = structmarshal.StructToMap(config)

	if err != nil {
		rerr = err
		return
	}

	return
}

func convertConfigFTS(inameBackend string, cb map[string]interface{}, exclusions []string) (res map[string]interface{}, inameAgent string, rerr error) {
	errStr := func(err string) {
		rerr = errors.New(err)
		return
	}
	inameAgent = "tfs-code"
	var config struct {
		URL        string `json:"url"`
		APIToken   string `json:"api_key"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		Collection string `json:"collection"`
		Hostname   string `json:"hostname"`
	}

	err := structmarshal.MapToStruct(cb, &config)
	if err != nil {
		rerr = err
		return
	}
	if config.URL == "" {
		errStr("missing url")
		return
	}
	if config.APIToken == "" {
		errStr("missing apitoken")
		return
	}
	if config.Username == "" {
		errStr("missing username")
		return
	}
	if config.Password == "" {
		errStr("missing password")
		return
	}
	if config.Collection == "" {
		config.Collection = "DefaultCollection"
	}
	res, err = structmarshal.StructToMap(config)
	if err != nil {
		rerr = err
		return
	}
	return
}
