package azureapi

import (
	"fmt"
	"sync"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/datamodel"
)

// Async simple async interface
type Async interface {
	Send(f func())
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

func (a *async) Send(f func()) {
	a.funcs <- f
}

func (a *async) Wait() {
	close(a.funcs)
	a.wg.Wait()
}

// AsyncProcessCallback callback function definition
type AsyncProcessCallback func(datamodel.Model)

// AsyncProcess proceses the channel reponse. Returns the channel to be used and the done chan bool
func AsyncProcess(name string, logger hclog.Logger, callback AsyncProcessCallback) (channel chan datamodel.Model, done chan bool) {
	channel = make(chan datamodel.Model)
	done = make(chan bool)
	go func() {
		logger.Info("started with " + name)
		count := 0
		for each := range channel {
			if callback != nil {
				callback(each)
			}
			count++
			if (count % 1000) == 0 {
				logger.Info(fmt.Sprintf("%d", count) + " " + name + " processed")
			}
		}
		logger.Info("ended with "+name, "count", count)
		done <- true
	}()
	return
}
