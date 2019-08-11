package cmdenroll

import (
	"context"
	"errors"
	"fmt"

	"github.com/pinpt/agent.next/pkg/encrypt"

	"github.com/pinpt/go-common/fileutil"

	"github.com/pinpt/agent.next/pkg/agentconf"
	"github.com/pinpt/agent.next/pkg/fsconf"

	"github.com/pinpt/agent.next/pkg/deviceinfo"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/event"
	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event/action"
	isdk "github.com/pinpt/integration-sdk"
)

type Opts struct {
	Logger       hclog.Logger
	PinpointRoot string
	Code         string
	Channel      string
	DeviceID     string
}

func Run(ctx context.Context, opts Opts) error {
	enr, err := newEnroller(opts)
	if err != nil {
		return err
	}
	return enr.Run(ctx)
}

type enroller struct {
	logger hclog.Logger
	opts   Opts
	fsconf fsconf.Locs
}

func newEnroller(opts Opts) (*enroller, error) {
	if opts.Code == "" {
		return nil, errors.New("provide enroll code")
	}
	if opts.Channel == "" {
		return nil, errors.New("provide enroll channel")
	}
	if opts.Channel == "" {
		return nil, errors.New("provide deviceID")
	}
	s := &enroller{}
	s.logger = opts.Logger
	s.opts = opts
	s.fsconf = fsconf.New(opts.PinpointRoot)
	return s, nil
}

func (s *enroller) Run(ctx context.Context) error {
	if fileutil.FileExists(s.fsconf.Config2) {
		return errors.New("agent is already enrolled")
	}

	done := make(chan error)
	go func() {
		res, err := s.WaitForResponse(ctx)
		if err != nil {
			done <- err
			return
		}
		err = s.ProcessResult(res)
		if err != nil {
			done <- err
			return
		}
		done <- nil
	}()

	err := s.SendEvent(ctx)
	if err != nil {
		return err
	}

	return <-done
}

func (s *enroller) SendEvent(ctx context.Context) error {
	s.logger.Debug("sending enroll event")

	data := agent.EnrollRequest{
		Code: s.opts.Code,
		UUID: s.opts.DeviceID,
	}

	deviceinfo.AppendCommonInfo(&data, "")

	reqEvent := event.PublishEvent{
		Object: &data,
		Headers: map[string]string{
			"uuid": s.opts.DeviceID,
		},
	}

	err := event.Publish(ctx, reqEvent, s.opts.Channel, "")
	if err != nil {
		return fmt.Errorf("could not send enroll event, err: %v", err)
	}

	return nil
}

type modelFactory struct {
}

func (f *modelFactory) New(name datamodel.ModelNameType) datamodel.Model {
	return isdk.New(name)
}

var factory action.ModelFactory = &modelFactory{}

func (s *enroller) WaitForResponse(ctx context.Context) (res agent.EnrollResponse, _ error) {

	errors := make(chan error, 1)

	enrollConfig := action.Config{
		GroupID: fmt.Sprintf("agent-%v", s.opts.DeviceID),
		Channel: s.opts.Channel,
		Factory: factory,
		Topic:   agent.EnrollResponseTopic.String(),
		Errors:  errors,
		Headers: map[string]string{
			"uuid": s.opts.DeviceID,
		},
	}

	done := make(chan bool)
	doneOnce := false

	cb := func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
		if doneOnce {
			return nil, nil
		}
		doneOnce = true

		defer func() {
			done <- true
		}()
		resp := instance.Object().(*agent.EnrollResponse)
		res = *resp
		return nil, nil
	}

	s.logger.Info("registering enroll")

	sub, err := action.Register(ctx, action.NewAction(cb), enrollConfig)
	if err != nil {
		panic(err)
	}

	go func() {
		for err := range errors {
			s.logger.Error("event subscription error", "err", err)
		}
	}()

	<-done

	err = sub.Close()
	if err != nil {
		s.logger.Info("could not close sub", "err", err)
	}

	return
}

func (s *enroller) ProcessResult(res agent.EnrollResponse) error {

	conf := agentconf.Config{}
	conf.APIKey = res.Apikey
	conf.CustomerID = res.CustomerID
	conf.Channel = s.opts.Channel
	conf.DeviceID = s.opts.DeviceID
	var err error
	conf.PPEncryptionKey, err = encrypt.GenerateKey()
	if err != nil {
		return err
	}

	err = agentconf.Save(conf, s.fsconf.Config2)
	if err != nil {
		return fmt.Errorf("could not save config, err: %v", err)
	}

	s.logger.Info("saved config into pinpoint dir", "dir", s.opts.PinpointRoot)

	return nil
}
