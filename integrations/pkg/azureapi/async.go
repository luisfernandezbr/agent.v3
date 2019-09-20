package azureapi

import (
	"fmt"
	"reflect"
	"sync"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/go-common/datamodel"
)

// Async simple async interface
type Async interface {
	Send(f AsyncMessage)
	Wait()
}

type AsyncMessage struct {
	Func func(interface{})
	Data interface{}
}

type async struct {
	funcs chan AsyncMessage
	wg    sync.WaitGroup
}

// NewAsync instantiates a new Async object
func NewAsync(concurrency int) Async {
	a := &async{}
	a.funcs = make(chan AsyncMessage, concurrency*2)
	a.wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			for each := range a.funcs {
				each.Func(each.Data)
			}
			a.wg.Done()
		}()
	}
	return a
}

func (a *async) Send(f AsyncMessage) {
	a.funcs <- f
}

func (a *async) Wait() {
	close(a.funcs)
	a.wg.Wait()
}

type AsyncProcessCallback func(datamodel.Model)

func AsyncProcess(name string, logger hclog.Logger, sender objsender.Sender, callback AsyncProcessCallback) (channel chan datamodel.Model, done chan bool) {
	channel = make(chan datamodel.Model)
	done = make(chan bool)
	go func() {
		logger.Info("started with " + name)
		count := 0
		for each := range channel {
			if sender != nil {
				if err := sender.Send(each); err != nil {
					logger.Error("error sending "+reflect.TypeOf(each).String(), "err", err)
				}
			}
			if callback != nil {
				callback(each)
			}
			count++
			if (count % 1000) == 0 {
				logger.Info(fmt.Sprintf("%d", count) + " " + name + " sent")
			}
		}
		logger.Info("ended with "+name, "count", count)
		done <- true
	}()
	return
}
