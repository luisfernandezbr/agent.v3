package cmdservicerunnorestarts

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/integration-sdk/agent"
)

func (s *runner) sendCrashes() error {
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
			s.logger.Error("could not upload service crash file", "n", f.Name())
		}
	}
	return nil
}

func (s *runner) sendCrashFile(loc string) error {
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

	s.deviceInfo.AppendCommonInfo(ev)
	err = s.sendEvent(ctx, ev, "", nil)
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
