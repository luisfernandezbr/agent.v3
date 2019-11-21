package fsqueue

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/pkg/fs"
)

type Request struct {
	Data Data
	Done chan (struct{})
}

type Data map[string]interface{}

type Queue struct {
	Input           chan Data
	forwardRequests chan Request

	logger hclog.Logger
	file   string

	pending map[int]Data
	idgen   int
	mu      sync.Mutex
}

func New(logger hclog.Logger, file string) (_ *Queue, forwardRequests chan Request, rerr error) {
	s := &Queue{}
	s.logger = logger
	s.file = file
	s.Input = make(chan Data)
	s.forwardRequests = make(chan Request, 10000)
	s.pending = map[int]Data{}

	err := s.readData()
	if err != nil {
		rerr = err
		return
	}

	return s, s.forwardRequests, nil
}

func (s *Queue) readData() error {
	b, err := ioutil.ReadFile(s.file)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	err = json.Unmarshal(b, &s.pending)
	if err != nil {
		return err
	}
	for id := range s.pending {
		if id > s.idgen {
			s.idgen = id
		}
	}
	return nil
}

func (s *Queue) addData(data Data) (id int, _ error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.idgen++
	id = s.idgen
	s.pending[id] = data
	return id, s.save()
}

func (s *Queue) dataDone(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.pending, id)
	return s.save()
}

func (s *Queue) save() error {
	b, err := json.Marshal(s.pending)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(s.file), 0777)
	if err != nil {
		return err
	}
	return fs.WriteToTempAndRename(bytes.NewReader(b), s.file)
}

func (s *Queue) Run(ctx context.Context) error {

	send := func(ctx context.Context, id int, data Data) {
		done := make(chan struct{})
		s.forwardRequests <- Request{Done: done, Data: data}
		go func() {
			select {
			case <-ctx.Done():
			case <-done:
				err := s.dataDone(id)
				if err != nil {
					s.logger.Error("could not mark export as done in fs", "err", err)
				}
			}
		}()
	}

	s.mu.Lock()
	for id, data := range s.pending {
		send(ctx, id, data)
	}
	s.mu.Unlock()

	for {
		select {
		case data := <-s.Input:
			id, err := s.addData(data)
			if err != nil {
				return err
			}
			send(ctx, id, data)
		case <-ctx.Done():
			return nil
		}
	}
}
