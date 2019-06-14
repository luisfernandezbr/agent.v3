package pservice

/*
import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/pinpt/go-common/log"
)

type restart struct {
	logger log.Logger
	run    Run

	fileLoc     string
	currentFile string

	watcher     *fsnotify.Watcher
	innerCancel func()
	innerDone   chan error
}

func RestartOnFileChanges(logger log.Logger, run Run, fileLoc string) Run {

	s := &restart{}
	s.logger = logger
	s.run = run
	s.fileLoc = fileLoc

	return func(ctx context.Context) error {

		var err error
		s.currentFile, err = s.readFile()
		if err != nil {
			return err
		}

		s.watcher, err = fsnotify.NewWatcher()
		if err != nil {
			return err
		}

		dir := filepath.Dir(s.fileLoc)
		err = s.watcher.Add(dir)
		if err != nil {
			return fmt.Errorf("file-watcher: error adding config file watcher err: %v", err)

		}

		s.innerDone, s.innerCancel = AsyncRun(ctx, s.run)

		for {
			select {
			case err := <-s.innerDone:
				log.Debug(s.logger, "file-watcher: received stop event, exiting")
				s.watcher.Close()
				return err
			case err := <-s.watcher.Errors:
				log.Error(s.logger, "file-watcher: error watching file", "err", err)
			case event := <-s.watcher.Events:
				if event.Name != s.fileLoc {
					continue
				}
				log.Info(s.logger, "file-watcher: event received for config file", "event", event)
				//if event.Op&fsnotify.Remove == 0 {
				//	log.Info(s.logger, "config file deleted")

				//	return
				newFile, err := s.readFile()
				if err != nil {
					log.Info(s.logger, "file-watcher: could not re-read the config file", "err", err)
					continue
				}
				if newFile == s.currentFile {
					log.Debug(s.logger, "file-watcher: config file did not change")
					//log.Debug(s.logger, "config file did not change", "v", []byte(newFile))
					continue
				}
				s.currentFile = newFile

				log.Info(s.logger, "file-watcher: restarting service due to changed config file")

				s.innerCancel()
				err = <-s.innerDone
				if err != nil {
					log.Info(s.logger, "file-watcher: failed to cancel cleanly while restarting", err, err)
				}

				s.innerDone, s.innerCancel = AsyncRun(ctx, s.run)
			}
		}

	}

}

func (s *restart) readFile() (string, error) {
	data, err := ioutil.ReadFile(s.fileLoc)
	if err != nil {
		return "", fmt.Errorf("could not start service, error reading file %v %v", s.fileLoc, err)
	}
	return string(data), nil
}
*/
