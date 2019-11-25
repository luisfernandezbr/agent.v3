package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	pstrings "github.com/pinpt/go-common/strings"

	"github.com/pinpt/agent.next/pkg/requests"
)

func TokenScopes(qc QueryContext) (scopes []string, rerr error) {
	u := pstrings.JoinURL(qc.APIURL3, "rate_limit")
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		rerr = err
		return
	}
	req.Header.Set("Authorization", "token "+qc.AuthToken)
	reqs := requests.New(qc.Logger, qc.Clients.TLSInsecure)
	resp, _, err := reqs.Do(context.TODO(), req)
	if err != nil {
		rerr = err
		return
	}
	scopesHeader := resp.Header.Get("X-OAuth-Scopes")
	if scopesHeader == "" {
		rerr = errors.New("X-OAuth-Scopes is empty")
		return
	}
	parts := strings.Split(scopesHeader, ",")
	for _, p := range parts {
		scopes = append(scopes, strings.TrimSpace(p))
	}
	return
}
