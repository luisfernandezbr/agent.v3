package pservice

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
	}
	for _, tc := range cases {
		res := delay(tc.In)
		if res != tc.Out {
			t.Errorf("invalid res for in %v, got %v, wanted %v", tc.In, res, tc.Out)
		}
	}
}

type testRun2 struct {
	t      *testing.T
	starts int
	mu     sync.Mutex
}

func (s *testRun2) Run(ctx context.Context) error {
	fmt.Println("tr: starting")
	s.mu.Lock()
	s.starts++
	if s.starts == 1 {
		s.mu.Unlock()
		return errors.New("start error")
	}
	s.mu.Unlock()
	fmt.Println("tr: started")
	<-ctx.Done()
	fmt.Println("tr: done")
	return nil
}

func (s *testRun2) Starts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.starts
}

func TestServiceRetryingStart(t *testing.T) {
	t.Skip("this test fails occasionally")

	ts := &testRun2{t: t}
	logger := &testLogger{t: t}
	run := Retrying(logger, ts.Run, func(retry int) time.Duration {
		return time.Millisecond
	})

	assert := assert.New(t)

	done, cancel := AsyncRunBg(run)
	fmt.Println("asyncRunBg completed")
	time.Sleep(10 * time.Millisecond)

	assert.Equal(2, ts.Starts())
	cancel()
	fmt.Println("cancel called")
	err := <-done
	assert.NoError(err)
}
