package api

import (
	"sync"
)

// Async simple async interface
type Async interface {
	Do(f func())
	Wait()
}

type async struct {
	funcs chan func()
	wg    sync.WaitGroup
}

// NewAsync instantiates a new Async object
func NewAsync(concurrency int) Async {
	a := &async{}
	a.funcs = make(chan func(), concurrency)
	a.wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			for f := range a.funcs {
				f()
			}
			a.wg.Done()
		}()
	}
	return a
}

func (a *async) Do(f func()) {
	a.funcs <- f
}

func (a *async) Wait() {
	close(a.funcs)
	a.wg.Wait()
}
