package cmdintegration

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/pinpt/agent/pkg/expin"
	"github.com/pinpt/agent/pkg/requests"
	"github.com/pinpt/go-common/api"
)

func oauthIntegrationNameToBackend(name string) string {
	switch name {
	case "jira-cloud":
		return "jira"
	case "bitbucket":
		return name
	default:
		panic(fmt.Errorf("oauth is not supported for integration: %v", name))
	}
}

func (s *Command) OAuthNewAccessToken(ii expin.Index) (accessToken string, _ error) {
	integrationName := s.IntegrationIDs[ii].Name

	if !s.Opts.AgentConfig.Backend.Enable {
		return "", errors.New("requested oauth access token, but Backend.Enable is false")
	}
	refresh := s.OAuthRefreshTokens[ii]
	if refresh == "" {
		return "", fmt.Errorf("requested oauth access token for integration %v, but we don't have refresh token for it", integrationName)
	}

	authAPIBase := api.BackendURL(api.AuthService, s.EnrollConf.Channel)

	// need oauth integration name
	url := authAPIBase + "oauth/" + oauthIntegrationNameToBackend(integrationName) + "/refresh/" + refresh

	s.Logger.Debug("requesting new oauth token from", "url", url)

	var res struct {
		AccessToken string `json:"access_token"`
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("could not create a url to get a new oauth token from pinpoint backend, err: %v", err)
	}

	reqs := requests.New(s.Logger, http.DefaultClient)

	_, err = reqs.JSON(req, &res)
	if err != nil {
		return "", fmt.Errorf("could not get new oauth token from pinpoint backend, err %v", err)
	}

	return res.AccessToken, nil
}
