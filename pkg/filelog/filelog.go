package filelog

import (
	"io"
	"os"
	"path/filepath"
)

type syncWriter struct {
	f *os.File
}

func NewSyncWriter(loc string) (io.Writer, error) {
	err := os.MkdirAll(filepath.Dir(loc), 0777)
	if err != nil {
		return nil, err
	}
	f, err := os.Create(loc)
	if err != nil {
		return nil, err
	}
	s := &syncWriter{}
	s.f = f
	return s, nil
}

func (s *syncWriter) Write(b []byte) (n int, _ error) {
	n, err := s.f.Write(b)
	if err != nil {
		return n, err
	}
	err = s.f.Sync()
	if err != nil {
		return n, err
	}
	return n, nil
}
