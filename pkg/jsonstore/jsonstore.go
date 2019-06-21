package jsonstore

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/pinpt/agent.next/pkg/fs"
)

type Store struct {
	loc  string
	data map[string]interface{}
	mu   sync.RWMutex
}

func New(loc string) (*Store, error) {
	s := &Store{}
	s.loc = loc
	s.data = map[string]interface{}{}

	b, err := ioutil.ReadFile(loc)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		return s, nil
	}

	return s, json.Unmarshal(b, &s.data)
}

func keyStr(key ...string) string {
	return strings.Join(key, "@")
}

func (s *Store) Get(key ...string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[keyStr(key...)]
}

func (s *Store) Set(val interface{}, key ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[keyStr(key...)] = val

	b, err := json.Marshal(s.data)
	if err != nil {
		return err
	}

	return fs.WriteToTempAndRename(bytes.NewReader(b), s.loc)
}
