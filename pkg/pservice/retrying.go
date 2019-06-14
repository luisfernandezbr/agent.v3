package pservice

import (
	"context"
	"math"
	"time"

	"github.com/pinpt/go-common/log"
)

// Delay returns the wait time for retry N. First retry = 1, second retry = 2.
// Initial attempt is not passed to this function and is always done immediately.
type RetryDelayFn func(retry int) time.Duration

func ExpRetryDelayFn(initialDelay time.Duration, maxDelay time.Duration, exponent float64) RetryDelayFn {
	return func(retry int) time.Duration {
		if retry == 0 {
			panic("retries start with 1")
		}
		res := float64(initialDelay) * math.Pow(exponent, float64(retry-1))
		res2 := time.Duration(res)
		if res2 > maxDelay {
			return maxDelay
		}
		return res2
	}
}

func Retrying(logger log.Logger, run Run, delay RetryDelayFn) Run {
	return func(ctx context.Context) error {
		i := 0
		for {
			log.Debug(logger, "retrying: starting service", "attempt", i)
			err := run(ctx)
			if err == nil {
				return nil
			}
			i++
			wait := delay(i)
			log.Error(logger, "retrying: service run returned an error", "err", err, "waiting", wait)
			timer := time.NewTimer(wait)
			select {
			case <-timer.C:
			case <-ctx.Done():
				return nil
			}
		}
	}
}
