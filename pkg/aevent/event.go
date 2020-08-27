// Package aevent contains agent default for publishing events
package aevent

import (
	"context"
	"time"

	"github.com/pinpt/go-common/v10/event"
)

const defaultPublishTimeout = 30 * time.Second

func Publish(ctx context.Context, ev event.PublishEvent, channel string, apiKey string, options ...event.Option) error {

	options2 := []event.Option{
		event.WithDeadline(time.Now().Add(defaultPublishTimeout)),
	}

	options2 = append(options2, options...)
	return event.Publish(ctx, ev, channel, apiKey, options2...)
}
