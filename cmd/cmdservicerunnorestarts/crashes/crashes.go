// Package crashes handle sending the crash logs to the server
package crashes

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/integration-sdk/agent"

	"github.com/pinpt/go-common/datamodel"
)

// CrashSender handles sending of crash logs to the server
type CrashSender struct {
	logger    hclog.Logger
	fsconf    fsconf.Locs
	sendEvent func(ctx context.Context, ev datamodel.Model) error
}

// New creates CrashSender
// sendEvent is the function that send the model to backend. It also has to append common device info.
func New(logger hclog.Logger, fsconf fsconf.Locs, sendEvent func(ctx context.Context, ev datamodel.Model) error) *CrashSender {
	s := &CrashSender{}
	s.logger = logger
	s.fsconf = fsconf
	s.sendEvent = sendEvent
	return s
}

// Send check the filesystem for crash logs and send all that are found.
// Only returns erros in case it was not possible to read the crash dir.
func (s *CrashSender) Send() error {
	dir := s.fsconf.ServiceRunCrashes
	files, err := ioutil.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, f := range files {
		err := s.sendCrashFile(filepath.Join(dir, f.Name()))
		if err != nil {
			s.logger.Error("could not upload service crash file", "n", f.Name(), "err", err)
		}
	}
	return nil
}

func (s *CrashSender) sendCrashFile(loc string) error {
	ctx := context.Background()
	n := filepath.Base(loc)
	if filepath.Ext(n) != ".log" {
		return nil
	}

	s.logger.Info("sending crash file", "f", n)
	b, err := ioutil.ReadFile(loc)
	if err != nil {
		return err
	}
	crashData := string(b)
	metaLoc := loc + ".json"
	b, err = ioutil.ReadFile(metaLoc)
	if err != nil {
		return err
	}
	var metaObj struct {
		CrashDate time.Time `json:"crash_date"`
	}
	err = json.Unmarshal(b, &metaObj)
	if err != nil {
		return err
	}
	ev := &agent.Crash{
		Data:      &crashData,
		Type:      agent.CrashTypeCrash,
		Component: "service",
	}

	date.ConvertToModel(metaObj.CrashDate, &ev.CrashDate)

	err = s.sendEvent(ctx, ev)
	if err != nil {
		return err
	}
	err = os.Remove(loc)
	if err != nil {
		return err
	}
	err = os.Remove(metaLoc)
	if err != nil {
		return err
	}
	return nil
}
