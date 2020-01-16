package cmdenroll

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	pstrings "github.com/pinpt/go-common/strings"

	"github.com/pinpt/agent/cmd/cmdvalidate"
	"github.com/pinpt/agent/pkg/date"
	"github.com/pinpt/agent/pkg/encrypt"
	"github.com/pinpt/agent/pkg/sysinfo"

	"github.com/pinpt/go-common/fileutil"

	"github.com/pinpt/agent/pkg/agentconf"
	"github.com/pinpt/agent/pkg/fsconf"

	"github.com/pinpt/agent/pkg/deviceinfo"

	"github.com/pinpt/go-common/event"
	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/event/action"
	isdk "github.com/pinpt/integration-sdk"
)

type Opts struct {
	Logger            hclog.Logger
	PinpointRoot      string
	IntegrationsDir   string
	Code              string
	Channel           string
	SkipValidate      bool
	SkipEnrollIfFound bool
}

func Run(ctx context.Context, opts Opts) error {
	enr, err := newEnroller(opts)
	if err != nil {
		return err
	}
	return enr.Run(ctx)
}

type enroller struct {
	logger            hclog.Logger
	opts              Opts
	fsconf            fsconf.Locs
	skipEnrollIfFound bool

	deviceID string
	systemID string
}

func newEnroller(opts Opts) (*enroller, error) {
	if opts.Code == "" {
		return nil, errors.New("provide enroll code")
	}
	if opts.Channel == "" {
		return nil, errors.New("provide enroll channel")
	}

	s := &enroller{}
	s.logger = opts.Logger
	s.opts = opts
	s.fsconf = fsconf.New(opts.PinpointRoot)
	s.deviceID = pstrings.NewUUIDV4()
	s.skipEnrollIfFound = opts.SkipEnrollIfFound
	var err error
	s.systemID, err = sysinfo.GetID2()
	if err != nil {
		s.logger.Info("using empty SystemID, could not get SystemID (PP_AGENT_ID)", "err", err)
		//return nil, fmt.Errorf("could not get SystemID (PP_AGENT_ID) err: %v", err)
	} else if s.systemID == "" {
		s.logger.Info("using empty SystemID, could not get SystemID (PP_AGENT_ID)")
		//return nil, errors.New("could not get SystemID (PP_AGENT_ID)")
	}
	return s, nil
}

func (s *enroller) Run(ctx context.Context) error {
	if fileutil.FileExists(s.fsconf.Config2) {
		if s.skipEnrollIfFound {
			s.logger.Info("skipping enroll, enroll config already exists")
			// if --skip-enroll-if-found we can safely ignore
			return nil
		}
		return errors.New("agent is already enrolled")
	}

	if !s.opts.SkipValidate {
		valid, err := cmdvalidate.Run(ctx, s.opts.Logger, s.opts.PinpointRoot)
		if err != nil {
			return err
		}
		if !valid {
			// to avoid stacking errors exiting instead of returning error
			os.Exit(1)
		}
	}

	done := make(chan error)
	ready := make(chan bool)
	go func() {
		res, err := s.WaitForResponse(ctx, ready)
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

	// wait for our subscriber to be registered
	<-ready

	// TODO: without this hardcoded delay enroll fails for cloud agent (not all the time thought)
	s.logger.Debug("Registered subscription, waiting for 5s")
	time.Sleep(5 * time.Second)

	err := s.SendEvent(ctx)
	if err != nil {
		return err
	}

	return <-done
}

func (s *enroller) SendEvent(ctx context.Context) error {
	s.logger.Debug("sending enroll event", "uuid", s.deviceID, "channel", s.opts.Channel)

	data := agent.EnrollRequest{
		Code: s.opts.Code,
		UUID: s.deviceID,
	}

	now := time.Now()
	date.ConvertToModel(now, &data.RequestDate)

	ci := deviceinfo.CommonInfo{
		DeviceID: s.deviceID,
		SystemID: s.systemID,
		Root:     s.opts.PinpointRoot,
	}
	ci.AppendCommonInfo(&data)

	reqEvent := event.PublishEvent{
		Object: &data,
		Headers: map[string]string{
			"uuid": s.deviceID,
		},
	}

	err := event.Publish(ctx, reqEvent, s.opts.Channel, "")
	if err != nil {
		return fmt.Errorf("could not send enroll event, err: %v", err)
	}
	s.logger.Debug("enroll event sent")

	return nil
}

type modelFactory struct {
}

func (f *modelFactory) New(name datamodel.ModelNameType) datamodel.Model {
	return isdk.New(name)
}

var factory action.ModelFactory = &modelFactory{}

func (s *enroller) WaitForResponse(ctx context.Context, ready chan<- bool) (res agent.EnrollResponse, _ error) {

	errors := make(chan error, 1)

	enrollConfig := action.Config{
		GroupID: fmt.Sprintf("agent-%v", s.deviceID),
		Channel: s.opts.Channel,
		Factory: factory,
		Topic:   agent.EnrollResponseTopic.String(),
		Errors:  errors,
		Headers: map[string]string{
			"uuid": s.deviceID,
		},
		Offset: "latest",
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

	s.logger.Info("registering enroll, waiting for response")

	sub, err := action.Register(ctx, action.NewAction(cb), enrollConfig)
	if err != nil {
		panic(err)
	}

	// wait for the subscription to be ready before sending any events
	sub.WaitForReady()
	ready <- true

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
	conf.DeviceID = s.deviceID
	conf.SystemID = deviceinfo.SystemID()
	conf.IntegrationsDir = s.opts.IntegrationsDir
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
