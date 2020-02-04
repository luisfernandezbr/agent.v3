package pservice

import (
	"context"
	"math"
	"time"

	"github.com/hashicorp/go-hclog"
)

// Run is the primary interface for service defintion. Easy to compose.
type Run func(context.Context) error

// AsyncRun wraps Run function in an async interaface that allows cancellation.
func AsyncRun(ctx context.Context, fn func(context.Context) error) (done chan error, cancel func()) {
	ctx, cancel = context.WithCancel(ctx)
	done = make(chan error)
	go func() {
		done <- fn(ctx)
	}()
	return done, cancel
}

// AsyncRunBg is similar to AsyncRun, but using context.Background
func AsyncRunBg(fn func(context.Context) error) (done chan error, cancel func()) {
	return AsyncRun(context.Background(), fn)
}

func Retrying(logger hclog.Logger, run Run, delay RetryDelayFn, resetFailuresAfter time.Duration) Run {
	logger = logger.Named("retrying")
	return func(ctx context.Context) error {
		i := 0
		for {
			logger.Info("starting service", "run", i)
			runStart := time.Now()
			err := run(ctx)
			if err != nil && err.Error() == "exit status 2" {
				logger.Info("exited from retrying")
				return err
			}
			runEnd := time.Now()
			dur := runEnd.Sub(runStart)
			logger.Info("service exited", "err", err, "dur", dur, "run", i)

			var wait time.Duration
			if dur > resetFailuresAfter {
				logger.Info("service was running longer than " + resetFailuresAfter.String() + " resetting run count")
				i = 0
			} else {
				i++
				wait = delay(i)
			}
			if err == nil {
				logger.Info("waiting to restart", "after", wait.String())
			} else {
				logger.Error("waiting to restart", "err", err, "after", wait.String())
			}
			timer := time.NewTimer(wait)
			select {
			case <-timer.C:
			case <-ctx.Done():
				return nil
			}
		}
	}
}

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
