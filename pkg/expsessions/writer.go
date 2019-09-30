package expsessions

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/io"
)

type Writer interface {
	// Write writes objects to output. Not required to be safe for concurrent use.
	Write(logger hclog.Logger, objs []map[string]interface{}) error
	// Close closes writer.
	Close() error
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

type FileWriter struct {
	id        ID
	modelType string
	outputDir string

	streamCreated sync.Once

	stream   *io.JSONStream
	streamMu sync.Mutex
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
	return s.stream.Close()
}

func (s *FileWriter) createStreamIfNeeded() error {
	if s.stream != nil {
		return nil
	}
	return s.createStream(s.outputDir)
}

func (s *FileWriter) createStream(outputDir string) error {
	base := strconv.FormatInt(time.Now().Unix(), 10) + "_" + strconv.Itoa(int(s.id)) + ".json.gz"
	fn := filepath.Join(outputDir, s.modelType, base)
	err := os.MkdirAll(filepath.Dir(fn), 0777)
	if err != nil {
		return err
	}
	stream, err := io.NewJSONStream(fn)
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
