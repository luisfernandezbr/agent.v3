package pservice

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
)

func TestServiceRetryingCancelImmediately(t *testing.T) {
	logger := hclog.New(hclog.DefaultOptions)

	runStart := make(chan bool)
	runEnd := make(chan error)

	run := func(ctx context.Context) error {
		fmt.Println("started")
		runStart <- true
		return <-runEnd
	}

	service := Retrying(logger, run, func(retry int) time.Duration {
		return time.Microsecond
	}, time.Hour)

	done, cancel := AsyncRunBg(service)
	<-runStart
	fmt.Println("cancel")
	cancel()
	runEnd <- nil
	err := <-done
	if err != nil {
		t.Fatal("should never return error", err)
	}
}

func TestServiceRetryingRetries(t *testing.T) {
	logger := hclog.New(hclog.DefaultOptions)

	runStart := make(chan bool)
	runEnd := make(chan error)

	run := func(ctx context.Context) error {
		fmt.Println("started")
		runStart <- true
		return <-runEnd
	}

	service := Retrying(logger, run, func(retry int) time.Duration {
		return time.Nanosecond
	}, time.Hour)

	done, cancel := AsyncRunBg(service)
	<-runStart
	runEnd <- nil
	<-runStart
	runEnd <- errors.New("e")
	<-runStart

	fmt.Println("cancel")
	cancel()
	runEnd <- nil

	err := <-done
	if err != nil {
		t.Fatal("should never return error", err)
	}
}

func TestServiceRetryingResetAfter(t *testing.T) {
	logger := hclog.New(hclog.DefaultOptions)

	runStart := make(chan bool)
	runEnd := make(chan error)

	run := func(ctx context.Context) error {
		fmt.Println("started")
		runStart <- true
		return <-runEnd
	}

	service := Retrying(logger, run, func(retry int) time.Duration {
		panic("should never run, since we reset to zero after Nanosecond")
	}, time.Nanosecond)

	done, cancel := AsyncRunBg(service)
	<-runStart
	time.Sleep(2 * time.Nanosecond)
	runEnd <- nil
	<-runStart
	fmt.Println("cancel")
	cancel()
	runEnd <- nil

	err := <-done
	if err != nil {
		t.Fatal("should never return error", err)
	}
}

func TestServiceRetryingCancelPropagation(t *testing.T) {
	logger := hclog.New(hclog.DefaultOptions)

	runStart := make(chan bool)
	runEnd := make(chan error)

	run := func(ctx context.Context) error {
		fmt.Println("started")
		runStart <- true
		<-ctx.Done()
		fmt.Println("received ctx done")
		return <-runEnd
	}

	service := Retrying(logger, run, func(retry int) time.Duration {
		return time.Nanosecond
	}, time.Hour)

	done, cancel := AsyncRunBg(service)
	<-runStart

	fmt.Println("cancel")
	cancel()
	runEnd <- nil

	err := <-done
	if err != nil {
		t.Fatal("should never return error", err)
	}
}

func TestExpRetryDelayFn(t *testing.T) {
	delay := ExpRetryDelayFn(3*time.Second, 60*time.Second, 2)

	cases := []struct {
		In  int
		Out time.Duration
	}{
		{In: 1, Out: 3 * time.Second},
		{In: 2, Out: 6 * time.Second},
		{In: 3, Out: 12 * time.Second},
		{In: 4, Out: 24 * time.Second},
		{In: 5, Out: 48 * time.Second},
		{In: 6, Out: 60 * time.Second},
		{In: 7, Out: 60 * time.Second},
	}
	for _, tc := range cases {
		res := delay(tc.In)
		if res != tc.Out {
			t.Errorf("invalid res for in %v, got %v, wanted %v", tc.In, res, tc.Out)
		}
	}
}
