package main

import (
	"github.com/pinpt/go-common/v10/datamodel"
	"github.com/pinpt/go-common/v10/event/action"
	isdk "github.com/pinpt/integration-sdk"
)

type agentOpts struct {
	DeviceID string
	Channel  string
}

type modelFactory struct {
}

func (f *modelFactory) New(name datamodel.ModelNameType) datamodel.Model {
	return isdk.New(name)
}

var factory action.ModelFactory = &modelFactory{}
