package exporter

import (
	"context"
	"fmt"
	"time"

	"github.com/pinpt/agent/pkg/structmarshal"
	"github.com/pinpt/go-common/v10/datetime"
)

// Run starts processing ExportQueue. This is a blocking call.
func (s *Exporter) Run() {
	go func() {
		for req := range s.queueRequestForwarder {
			req2 := Request{}
			err := structmarshal.MapToStruct(req.Data, &req2)
			if err != nil {
				s.logger.Error("could not unmarshal export request from map", "err", err)
			}
			s.setRunning(true)
			s.export(req2.Data, req2.MessageID)
			s.setRunning(false)
			req.Done <- struct{}{}
		}
	}()

	go func() {
		err := s.queue.Run(context.Background())
		if err != nil {
			panic(err)
		}
	}()

	for req := range s.ExportQueue {
		data := req.Data

		handleError := func(err error) {
			s.logger.Error("export finished with error", "err", err)
			err2 := s.sendFailedEvent(data.JobID, time.Now(), time.Now(), err)
			if err2 != nil {
				s.logger.Error("error sending failed export event", "sending_err", err2, "export_err", err)
			}
		}

		// have handling of request deadline before we save the request on disk, otherwise requests would not be retried if failed
		// need deadline here to prevent a bunch of older requests from queue being accepted
		requestDate := datetime.DateFromEpoch(data.RequestDate.Epoch)
		const exportEventDeadline = 5 * time.Minute
		if requestDate.Before(time.Now().Add(-exportEventDeadline)) {
			handleError(fmt.Errorf("export request date is older than deadline, ignoring. deadline: %v", exportEventDeadline.String()))
			continue
		}

		m, err := structmarshal.StructToMap(req)
		if err != nil {
			handleError(fmt.Errorf("could not marshal export request to map: %v", err))
			continue
		}

		s.queue.Input <- m
	}
	return
}
