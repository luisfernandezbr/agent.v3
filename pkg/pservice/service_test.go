package pservice

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

type testLogger struct {
	t *testing.T
}

func (s *testLogger) Log(keyvals ...interface{}) error {
	//s.t.Log(keyvals...)
	fmt.Println(keyvals...)
	return nil
}

type tempFs struct {
	d    string
	File string
}

func newTempFs() *tempFs {
	d, err := ioutil.TempDir("", "pp-test")
	if err != nil {
		panic(err)
	}
	s := &tempFs{d: d}
	s.File = filepath.Join(s.d, "file")
	err = ioutil.WriteFile(s.File, nil, 0666)
	if err != nil {
		panic(err)
	}
	return s
}

func (s *tempFs) Remove() {
	err := os.RemoveAll(s.d)
	if err != nil {
		panic(err)
	}
}
