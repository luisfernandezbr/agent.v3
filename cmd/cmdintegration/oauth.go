package cmdintegration

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/pinpt/agent/pkg/requests2"

	"github.com/pinpt/agent/pkg/expin"
	"github.com/pinpt/go-common/api"
)

func oauthIntegrationNameToBackend(name string) string {
	switch name {
	case "jira-cloud":
		return "jira"
	case "bitbucket", "office365":
		return name
	case "gcal":
		return "gsuite"
	default:
		panic(fmt.Errorf("oauth is not supported for integration: %v", name))
	}
}

func (s *Command) OAuthNewAccessTokenFromRefreshToken(integrationName string, refresh string) (accessToken string, rerr error) {
	authAPIBase := api.BackendURL(api.AuthService, s.EnrollConf.Channel)

	// need oauth integration name
	url := authAPIBase + "oauth/" + oauthIntegrationNameToBackend(integrationName) + "/refresh/" + url.PathEscape(refresh)

	s.Logger.Debug("requesting new oauth token from", "url", url)

	var res struct {
		AccessToken string `json:"access_token"`
	}

	req := requests2.NewRequest()
	req.URL = url
	reqs := requests2.New(s.Logger, http.DefaultClient)
	_, err := reqs.JSON(req, res)
	if err != nil {
		rerr = fmt.Errorf("could not get new oauth token from pinpoint backend, err %v", err)
		return
	}

	return res.AccessToken, nil
}

func (s *Command) OAuthNewAccessToken(exp expin.Export) (accessToken string, _ error) {
	integration := s.Integrations[exp]
	integrationName := exp.IntegrationDef.Name

	if !s.Opts.AgentConfig.Backend.Enable {
		return "", errors.New("requested oauth access token, but Backend.Enable is false")
	}
	refresh := integration.OauthRefreshToken
	if refresh == "" {
		return "", fmt.Errorf("requested oauth access token for integration %v, but we don't have refresh token for it", integrationName)
	}

	return s.OAuthNewAccessTokenFromRefreshToken(integrationName, refresh)
}
