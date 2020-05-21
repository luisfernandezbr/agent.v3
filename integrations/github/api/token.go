package api

import (
	"context"
	"errors"
	"strings"

	"github.com/pinpt/agent/pkg/requests2"
)

func TokenScopes(qc QueryContext) (scopes []string, rerr error) {
	req := newRestRequest(qc, "rate_limit")
	reqs := requests2.New(qc.Logger, qc.Clients.TLSInsecure)
	resp, err := reqs.Do(context.TODO(), req)
	if err != nil {
		rerr = err
		return
	}
	scopesHeader := resp.Resp.Header.Get("X-OAuth-Scopes")
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
