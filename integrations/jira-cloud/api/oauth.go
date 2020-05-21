package api

import (
	"errors"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/reqstats"
	"github.com/pinpt/agent/pkg/requests"
)

type Site struct {
	ID  string
	URL string
}

func AccessibleResources(
	logger hclog.Logger,
	hc reqstats.Clients,
	accessToken string) (res []Site, rerr error) {

	req := requests.NewRequest()
	req.URL = "https://api.atlassian.com/oauth/token/accessible-resources"
	req.Header.Set("Authorization", "Bearer "+accessToken)
	reqs := requests.New(logger, hc.Default)

	resp, err := reqs.JSON(req, &res)
	if resp.Resp != nil && resp.Resp.StatusCode == 401 {
		rerr = errors.New("Auth token provided is not correct. Getting status 401 when trying to call api.atlassian.com/oauth/token/accessible-resources.")
		return
	}
	if err != nil {
		rerr = err
		return
	}

	return
}
