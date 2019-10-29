package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/reqstats"
	"github.com/pinpt/agent.next/pkg/requests"
)

type Site struct {
	ID  string
	URL string
}

func AccessibleResources(
	logger hclog.Logger,
	hc reqstats.Clients,
	accessToken string) (res []Site, rerr error) {

	// retry 10 times, 500 millis per retry, and max timeout 1 minute
	reqs := requests.NewRetryable(logger, hc.Default, 10, 500*time.Millisecond, 1*time.Minute)

	req, err := http.NewRequest(http.MethodGet, "https://api.atlassian.com/oauth/token/accessible-resources", nil)
	if err != nil {
		rerr = err
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := reqs.JSON(req, &res)
	if resp != nil && resp.StatusCode == 401 {
		rerr = errors.New("Auth token provided is not correct. Getting status 401 when trying to call api.atlassian.com/oauth/token/accessible-resources.")
		return
	}
	if err != nil {
		rerr = err
		return
	}

	return
}
