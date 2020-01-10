package expsessions

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"sync"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/fs"
)

type WriterDedup struct {
	wr        Writer
	ds        DedupStore
	modelName string
}

func NewWriterDedup(wr Writer, ds DedupStore, modelName string) *WriterDedup {
	s := &WriterDedup{}
	s.wr = wr
	s.ds = ds
	s.modelName = modelName
	return s
}

func (s *WriterDedup) Write(logger hclog.Logger, objs []map[string]interface{}) error {
	var filtered []map[string]interface{}
	for _, obj := range objs {
		wasAlreadySent, err := s.ds.MarkAsSent(obj, s.modelName)
		if err != nil {
			return err
		}
		if !wasAlreadySent {
			filtered = append(filtered, obj)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return s.wr.Write(logger, filtered)
}

func (s *WriterDedup) Close() error {
	return s.wr.Close()
}

func (s *WriterDedup) Rollback() error {
	return s.wr.Rollback()
}

type DedupStore interface {
	// MarkAsSent marks the object as sent, if it wasn't already.
	// And returns the bool if it was already sent before.
	// Safe for concurrent use.
	MarkAsSent(obj map[string]interface{}, modelName string) (wasAlreadySent bool, _ error)

	Save() error

	Stats() (new int, dups int)
}

type dedupStore struct {
	loc string

	mu sync.Mutex
	// map[ref_type][model_name][id][data_hashcode]
	data map[string]map[string]map[string]string

	dups int
	new  int
}

func NewDedupStore(loc string) (DedupStore, error) {

	s := &dedupStore{}
	s.loc = loc
	s.data = map[string]map[string]map[string]string{}

	b, err := ioutil.ReadFile(loc)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		return s, nil
	}

	return s, json.Unmarshal(b, &s.data)
}

func (s *dedupStore) MarkAsSent(obj map[string]interface{}, modelName string) (wasAlreadySent bool, rerr error) {

	refType, ok := obj["ref_type"].(string)
	if !ok || refType == "" {
		rerr = errors.New("dedupStore: passed object does not have ref_type")
		return
	}
	id, ok := obj["id"].(string)
	if !ok {
		rerr = errors.New("dedupStore: passed object does not have id")
		return
	}
	hashcode, ok := obj["hashcode"].(string)
	if !ok {
		rerr = errors.New("dedupStore: passed object does not have hashcode")
		return
	}
	s.mu.Lock()
	if _, ok := s.data[refType]; !ok {
		s.data[refType] = map[string]map[string]string{}
	}
	if _, ok := s.data[refType][modelName]; !ok {
		s.data[refType][modelName] = map[string]string{}
	}
	prev := s.data[refType][modelName][id]
	s.data[refType][modelName][id] = hashcode
	dup := prev == hashcode

	if dup {
		s.dups++
	} else {
		s.new++
	}
	s.mu.Unlock()
	return dup, nil
}

func (s *dedupStore) Stats() (new int, dups int) {
	return s.new, s.dups
}

func (s *dedupStore) Save() error {
	b, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	return fs.WriteToTempAndRename(bytes.NewReader(b), s.loc)
}
