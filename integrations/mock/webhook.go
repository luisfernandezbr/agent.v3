package main

import (
	"context"
	"errors"

	"github.com/pinpt/agent/rpcdef"
)

func (s *Integration) Webhook(ctx context.Context, headers map[string]string, body string, config rpcdef.ExportConfig) (res rpcdef.WebhookResult, rerr error) {
	rerr = errors.New("webhook not supported")
	return
}
