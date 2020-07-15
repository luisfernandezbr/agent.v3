package expsessions

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/v10/io"
)

type Writer interface {
	// Write writes objects to output. Not required to be safe for concurrent use.
	Write(logger hclog.Logger, objs []map[string]interface{}) error
	// Close closes writer.
	Close() error

	Rollback() error
}

type MockWriter struct {
	Data   []map[string]interface{}
	Closed bool
}

func NewMockWriter() *MockWriter {
	s := &MockWriter{}
	return s
}

func (s *MockWriter) Write(logger hclog.Logger, objs []map[string]interface{}) error {
	s.Data = append(s.Data, objs...)
	return nil
}

func (s *MockWriter) Close() error {
	s.Closed = true
	return nil
}

func (s *MockWriter) Rollback() error {
	return nil
}

type MockWriters struct {
	wr map[string]*MockWriter
	mu sync.Mutex
}

func NewMockWriters() *MockWriters {
	return &MockWriters{
		wr: map[string]*MockWriter{},
	}
}

func (s *MockWriters) NewWriter(modelName string, id ID) Writer {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.wr[modelName]; !ok {
		s.wr[modelName] = NewMockWriter()
	}
	return s.wr[modelName]
}

func (s *MockWriters) DataByModel(modelName string) (res []map[string]interface{}) {
	for _, m := range s.wr[modelName].Data {
		res = append(res, m)
	}
	sort.Slice(res, func(i, j int) bool {
		a := res[i]["id"].(string)
		b := res[j]["id"].(string)
		return a < b
	})
	return
}

func (s *MockWriters) Data() map[string][]map[string]interface{} {
	res := map[string][]map[string]interface{}{}
	for modelName := range s.wr {
		res[modelName] = s.DataByModel(modelName)
	}
	return res
}

type FileWriter struct {
	id        ID
	modelType string
	outputDir string

	streamCreated sync.Once

	stream   *io.JSONStream
	streamMu sync.Mutex

	loc string
}

func NewFileWriter(modelType string, outputDir string, id ID) *FileWriter {
	s := &FileWriter{}
	s.id = id
	s.modelType = modelType
	s.outputDir = outputDir
	return s
}

func (s *FileWriter) Close() error {
	if s.stream == nil {
		// there was no stream, since no objects were sent
		return nil
	}
	err := s.stream.Close()
	if err != nil {
		return err
	}

	return os.Rename(s.loc+".temp.gz", s.loc)
}

func (s *FileWriter) Rollback() error {
	if s.stream == nil {
		// there was no stream, since no objects were sent
		return nil
	}
	err := s.stream.Close()
	if err != nil {
		return err
	}
	return os.Remove(s.loc + ".temp.gz")
}

func (s *FileWriter) createStreamIfNeeded() error {
	if s.stream != nil {
		return nil
	}
	return s.createStream(s.outputDir)
}

func (s *FileWriter) createStream(outputDir string) error {
	base := strconv.FormatInt(time.Now().Unix(), 10) + "_" + strconv.Itoa(int(s.id)) + ".json.gz"
	s.loc = filepath.Join(outputDir, s.modelType, base)
	err := os.MkdirAll(filepath.Dir(s.loc), 0777)
	if err != nil {
		return err
	}
	stream, err := io.NewJSONStream(s.loc + ".temp.gz")
	if err != nil {
		return err
	}
	s.stream = stream
	return nil
}

func (s *FileWriter) Write(logger hclog.Logger, objs []map[string]interface{}) error {
	err := s.createStreamIfNeeded()
	if err != nil {
		return err
	}
	for _, obj := range objs {
		err := s.stream.Write(obj)
		if err != nil {
			return err
		}
	}
	return nil
}
