package azurecommon

import (
	"fmt"
	"reflect"

	"github.com/pinpt/agent.next/pkg/objsender"
	"github.com/pinpt/go-common/datamodel"
)

func (s *Integration) execute(name string, sender objsender.Sender) (channel chan datamodel.Model, done chan bool) {
	channel = make(chan datamodel.Model)
	done = make(chan bool)
	go func() {
		s.logger.Info("started with " + name)
		count := 0
		for each := range channel {
			if err := sender.Send(each); err != nil {
				s.logger.Error("error sending "+reflect.TypeOf(each).String(), "err", err)
			}
			count++
			if (count % 1000) == 0 {
				s.logger.Info(fmt.Sprintf("%d", count) + " " + name + " sent")
			}
		}
		s.logger.Info("ended with "+name, "count", count)
		done <- true
	}()
	return
}
