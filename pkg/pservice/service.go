package pservice

import "context"

// Run is the primary interface for service defintion. Easy to compose.
type Run func(context.Context) error

func AsyncRun(ctx context.Context, fn func(context.Context) error) (done chan error, cancel func()) {
	ctx, cancel = context.WithCancel(ctx)
	done = make(chan error)
	go func() {
		done <- fn(ctx)
	}()
	return done, cancel
}

func AsyncRunBg(fn func(context.Context) error) (done chan error, cancel func()) {
	return AsyncRun(context.Background(), fn)
}
