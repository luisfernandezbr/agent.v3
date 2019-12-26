package inconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/pinpt/agent/cmd/cmdintegration"
	azureapi "github.com/pinpt/agent/integrations/azuretfs/api"

	"github.com/pinpt/agent/pkg/encrypt"
	"github.com/pinpt/agent/pkg/integrationid"
	"github.com/pinpt/agent/pkg/structmarshal"
)

// ConfigFromEvent converts the config received from backend export or onboarding events and
// converts it to config that integrations can accept (cmdintegration.Integration).
// Also requires encryptionKey to decrypt the auth data.
func ConfigFromEvent(data map[string]interface{},
	systemType IntegrationType,
	encryptionKey string) (res cmdintegration.Integration, rerr error) {
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
		rerr = fmt.Errorf("could not decrypt Authorization field in event for %v integration: %v", obj.Name, err)
		return
	}

	var authObj map[string]interface{}
	err = json.Unmarshal([]byte(auth), &authObj)
	if err != nil {
		rerr = err
		return
	}

	configAgent, agentIn, err := ConvertConfig(obj.Name, systemType, authObj, obj.Exclusions)
	if err != nil {
		rerr = fmt.Errorf("config object in event is not valid: %v", err)
		return
	}
	res.Name = agentIn.Name
	res.Type = agentIn.Type
	res.Config = configAgent

	return
}

type agentIntegration struct {
	Name string
	Type string
}

// ConvertConfig is similar to ConfigFromEvent, both convert backend config to integration config. But if ConfigFromEvent requires encryption key, ConvertConfig can process the configuration passed directly, which we use in validate.
func ConvertConfig(integrationNameBackend string, systemTypeBackend IntegrationType, configBackend map[string]interface{}, exclusions []string) (configAgent map[string]interface{}, agentIn agentIntegration, rerr error) {

	var fn func(integrationNameBackend string, systemTypeBackend IntegrationType, configBackend map[string]interface{}, exclusions []string) (configAgent map[string]interface{}, agentIn agentIntegration, rerr error)

	switch integrationNameBackend {
	case "github":
		fn = convertConfigGithub
	case "bitbucket":
		fn = convertConfigBitbucket
	case "gitlab":
		fn = convertConfigGitlab
	case "jira":
		fn = convertConfigJira
	case "sonarqube":
		fn = convertConfigSonarqube
	case "tfs", "azure":
		fn = convertConfigAzureTFS
	case "workday":
		fn = convertConfigWorkday
	default:
		rerr = fmt.Errorf("unsupported integration: %v", integrationNameBackend)
		return
	}

	configAgent, agentIn, rerr = fn(integrationNameBackend, systemTypeBackend, configBackend, exclusions)

	if agentIn.Name == "" {
		agentIn.Name = integrationNameBackend
	}

	return
}

func convertConfigGithub(integrationNameBackend string, systemTypeBackend IntegrationType, cb map[string]interface{}, exclusions []string) (res map[string]interface{}, agentIn agentIntegration, rerr error) {

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

func convertConfigGitlab(integrationNameBackend string, systemTypeBackend IntegrationType, cb map[string]interface{}, exclusions []string) (res map[string]interface{}, agentIn agentIntegration, rerr error) {

	errStr := func(err string) {
		rerr = errors.New(err)
		return
	}

	res = map[string]interface{}{}

	var config struct {
		URL           string   `json:"url"`
		APIToken      string   `json:"apitoken"`
		ExcludedRepos []string `json:"excluded_repos"`
		AccessToken   string   `json:"access_token"`
	}

	err := structmarshal.MapToStruct(cb, &config)
	if err != nil {
		rerr = err
		return
	}

	accessToken, _ := cb["access_token"].(string)

	if accessToken != "" {
		// this is gitlab.com cloud auth
		config.AccessToken = accessToken
		config.URL = "https://gitlab.com"
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

func convertConfigBitbucket(integrationNameBackend string, systemTypeBackend IntegrationType, cb map[string]interface{}, exclusions []string) (res map[string]interface{}, agentIn agentIntegration, rerr error) {

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
		AccessToken   string   `json:"access_token"`
		RefreshToken  string   `json:"refresh_token"`
	}

	err := structmarshal.MapToStruct(cb, &config)
	if err != nil {
		rerr = err
		return
	}

	accessToken, _ := cb["access_token"].(string)

	if accessToken != "" {
		// this is bitbucket.org cloud auth
		config.URL = "https://api.bitbucket.org"
		config.AccessToken = accessToken
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

func convertConfigJira(integrationNameBackend string, systemTypeBackend IntegrationType, cb map[string]interface{}, exclusions []string) (res map[string]interface{}, agentIn agentIntegration, rerr error) {
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
	agentIn.Name = "jira-hosted"
	if strings.HasSuffix(u.Host, ".atlassian.net") {
		agentIn.Name = "jira-cloud"
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

func convertConfigSonarqube(integrationNameBackend string, systemTypeBackend IntegrationType, cb map[string]interface{}, exclusions []string) (res map[string]interface{}, agentIn agentIntegration, rerr error) {
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

func convertConfigAzureTFS(integrationNameBackend string, systemTypeBackend IntegrationType, cb map[string]interface{}, exclusions []string) (res map[string]interface{}, agentIn agentIntegration, rerr error) {
	errStr := func(err string) {
		rerr = errors.New(err)
		return
	}
	isazure := strings.HasPrefix(integrationNameBackend, "azure")
	var conf struct {
		RefType          string         `json:"reftype"` // azure or tfs
		IntegrationType  string         `json:"type"`    // sourcecode or work
		Credentials      azureapi.Creds `json:"credentials"`
		ExcludedRepos    []string       `json:"excluded_repos"`
		ExcludedProjects []string       `json:"excluded_projects"`
	}
	if rerr = structmarshal.MapToStruct(cb, &conf.Credentials); rerr != nil {
		return
	}
	if conf.Credentials.APIKey == "" {
		errStr("missing api_key")
		return
	}
	if conf.Credentials.URL == "" {
		errStr("missing url")
		return
	}
	if isazure {
		if conf.Credentials.Organization == nil {
			errStr("missing organization")
			return
		}
	} else {
		if conf.Credentials.CollectionName == nil {
			errStr("missing collection")
			return
		}
	}
	conf.RefType = integrationNameBackend
	conf.IntegrationType = systemTypeBackend.String()

	agentIn.Name = "azuretfs"
	switch systemTypeBackend {
	case IntegrationTypeSourcecode:
		conf.ExcludedRepos = exclusions
		agentIn.Type = integrationid.TypeSourcecode
	case IntegrationTypeWork:
		conf.ExcludedProjects = exclusions
		agentIn.Type = integrationid.TypeWork
	default:
		rerr = fmt.Errorf("invalid systemtype received from backend: %v", systemTypeBackend)
		return
	}
	res, rerr = structmarshal.StructToMap(conf)
	return
}

func convertConfigWorkday(integrationNameBackend string, systemTypeBackend IntegrationType, cb map[string]interface{}, exclusions []string) (res map[string]interface{}, agentIn agentIntegration, rerr error) {
	errStr := func(err string) {
		rerr = errors.New(err)
		return
	}
	var config struct {
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
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
	if config.Username == "" {
		errStr("missing username")
		return
	}
	if config.Password == "" {
		errStr("missing password")
		return
	}
	res, err = structmarshal.StructToMap(config)
	if err != nil {
		rerr = err
		return
	}
	return
}
