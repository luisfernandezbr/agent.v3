package filestore

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pinpt/agent/pkg/fs"
)

// Store saves and marshals larger objects to the filesystem
// Not safe for concurrent use with the same key
type Store interface {
	Set(k string, obj interface{}) error
	Get(k string, obj interface{}) error
}

type FileStore struct {
	loc string
}

func New(loc string) *FileStore {
	return &FileStore{
		loc: loc,
	}
}

func (s *FileStore) Set(k string, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	loc := s.keyToPath(k)
	err = os.MkdirAll(filepath.Dir(loc), 0777)
	if err != nil {
		return err
	}
	return fs.WriteToTempAndRename(bytes.NewReader(b), loc)
}

func (s *FileStore) Get(k string, obj interface{}) error {
	b, err := ioutil.ReadFile(s.keyToPath(k))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &obj)
}

func (s *FileStore) keyToPath(k string) string {
	return filepath.Join(s.loc, filepath.FromSlash(k))
}
