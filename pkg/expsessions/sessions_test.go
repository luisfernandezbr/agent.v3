package expsessions

import (
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/pkg/expin"
	"github.com/stretchr/testify/assert"
)

type lastProcessedMock map[string]interface{}

func (s lastProcessedMock) Get(key ...string) interface{} {
	res := s[strings.Join(key, "@")]
	return res
}

func (s lastProcessedMock) Set(value interface{}, key ...string) error {
	s[strings.Join(key, "@")] = value
	return nil
}

var testIn = expin.Export{IntegrationID: "1", IntegrationDef: inconfig.IntegrationDef{Name: "in1"}}

var inPre = testIn.IntegrationDef.String() + "/"

func TestExpSessionsBasic(t *testing.T) {
	opts := Opts{}
	opts.Logger = hclog.Default()
	opts.NewWriter = func(modelType string, id ID) Writer {
		return NewMockWriter()
	}

	lpm := lastProcessedMock{}
	opts.LastProcessed = lpm
	m := New(opts)

	id, _, err := m.SessionRoot(testIn, "m1")
	if err != nil {
		t.Fatal(err)
	}
	err = m.Write(id, []map[string]interface{}{{"k1": "v1"}})
	if err != nil {
		t.Fatal(err)
	}
	err = m.Done(id, "id1")
	if err != nil {
		t.Fatal(err)
	}
}

func TestExpSessionsWrittenData(t *testing.T) {
	opts := Opts{}
	opts.Logger = hclog.Default()

	var mockWriter *MockWriter
	opts.NewWriter = func(modelType string, id ID) Writer {
		if modelType != "m1" || id != 1 {
			t.Fatal("invalid modelType or id")
		}
		mockWriter = NewMockWriter()
		return mockWriter
	}

	m := New(opts)

	id, _, err := m.SessionRoot(testIn, "m1")
	if err != nil {
		t.Fatal(err)
	}

	err = m.Write(id, []map[string]interface{}{{"k1": "v1"}})
	if err != nil {
		t.Fatal(err)
	}
	err = m.Write(id, []map[string]interface{}{{"k2": "v2"}})
	if err != nil {
		t.Fatal(err)
	}

	err = m.Done(id, "id1")
	if err != nil {
		t.Fatal(err)
	}

	assert := assert.New(t)
	assert.Equal([]map[string]interface{}{{"k1": "v1"}, {"k2": "v2"}}, mockWriter.Data)
	assert.Equal(true, mockWriter.Closed)
}

func TestExpSessionsLastProcessed(t *testing.T) {
	opts := Opts{}
	opts.Logger = hclog.Default()

	opts.NewWriter = func(modelType string, id ID) Writer {
		return NewMockWriter()
	}

	lpm := lastProcessedMock{}
	opts.LastProcessed = lpm
	m := New(opts)

	id, lp1, err := m.SessionRoot(testIn, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if lp1 != nil {
		t.Fatal("expected last processed to be nil initially")
	}

	err = m.Done(id, "id1")
	if err != nil {
		t.Fatal(err)
	}
	id, lp1, err = m.SessionRoot(testIn, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := lp1.(string); v != "id1" {
		t.Fatal("expected last processed to be id1 on second run")
	}
}

func TestExpSessionsLastProcessedNested(t *testing.T) {
	opts := Opts{}
	opts.Logger = hclog.Default()

	opts.NewWriter = func(modelType string, id ID) Writer {
		return NewMockWriter()
	}

	lpm := lastProcessedMock{}
	opts.LastProcessed = lpm
	m := New(opts)

	m1, _, err := m.SessionRoot(testIn, "m1")
	if err != nil {
		t.Fatal(err)
	}

	m2, _, err := m.Session("m2", m1, "m1obj1", "")
	if err != nil {
		t.Fatal(err)
	}

	lpm.Set("id1", inPre+"m1/m1obj1/m2/m2obj1/m3")

	m3, lp, err := m.Session("m3", m2, "m2obj1", "")
	if err != nil {
		t.Fatal(err)
	}

	assert := assert.New(t)
	assert.Equal("id1", lp)

	err = m.Done(m3, "id1v2")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal("id1v2", lpm.Get(inPre+"m1/m1obj1/m2/m2obj1/m3"))

	lpm.Set("id2", inPre+"m1/m1obj1/m2/m2obj2/m3")

	_, lp, err = m.Session("m3", m2, "m2obj2", "")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal("id2", lp)

}

func TestExpSessionsProgress(t *testing.T) {
	opts := Opts{}
	opts.Logger = hclog.Default()

	opts.NewWriter = func(modelType string, id ID) Writer {
		return NewMockWriter()
	}
	current := 0
	total := 0

	opts.SendProgress = func(pp ProgressPath, c, to int) {
		if pp.String() != inPre+"m1" {
			t.Fatal("invalid progress path")
		}
		current = c
		total = to
	}

	m := New(opts)

	id, _, err := m.SessionRoot(testIn, "m1")
	if err != nil {
		t.Fatal(err)
	}
	assert := assert.New(t)

	m.Progress(id, 1, 10)
	assert.Equal(1, current)
	assert.Equal(10, total)
	m.Progress(id, 2, 10)
	assert.Equal(2, current)
	assert.Equal(10, total)

	err = m.Write(id, []map[string]interface{}{{"k": "v"}})
	if err != nil {
		t.Fatal(err)
	}

	err = m.Done(id, "")
	if err != nil {
		t.Fatal(err)
	}

	// after calling done updates current and total with actual values sent
	assert.Equal(1, current)
	assert.Equal(1, total)
}

func TestExpSessionsProgressNested(t *testing.T) {
	opts := Opts{}
	opts.Logger = hclog.Default()

	opts.NewWriter = func(modelType string, id ID) Writer {
		return NewMockWriter()
	}
	current := 0
	total := 0

	opts.SendProgress = func(pp ProgressPath, c, to int) {
		if pp.String() != inPre+"m1/m1obj1/m2" {
			t.Fatal("invalid progress path")
		}
		current = c
		total = to
	}

	m := New(opts)

	m1, _, err := m.SessionRoot(testIn, "m1")
	if err != nil {
		t.Fatal(err)
	}

	m2, _, err := m.Session("m2", m1, "m1obj1", "")
	if err != nil {
		t.Fatal(err)
	}

	assert := assert.New(t)

	m.Progress(m2, 1, 10)
	assert.Equal(1, current)
	assert.Equal(10, total)
	m.Progress(m2, 2, 10)
	assert.Equal(2, current)
	assert.Equal(10, total)

	err = m.Write(m2, []map[string]interface{}{{"k": "v"}})
	if err != nil {
		t.Fatal(err)
	}

	err = m.Done(m2, "")
	if err != nil {
		t.Fatal(err)
	}

	// after calling done updates current and total with actual values sent
	assert.Equal(1, current)
	assert.Equal(1, total)
}

// Check that tracking sessions do not support writes
func TestExpSessionsTracking(t *testing.T) {
	opts := Opts{}
	opts.Logger = hclog.Default()
	opts.NewWriter = func(modelType string, id ID) Writer {
		return NewMockWriter()
	}
	m := New(opts)

	id, _, err := m.SessionRootTracking(testIn, "m1")
	if err != nil {
		t.Fatal(err)
	}
	err = m.Write(id, []map[string]interface{}{{"k1": "v1"}})
	if err == nil {
		t.Fatal("expected an error trying to write on tracking session, got no error")
	}
	t.Log("expected error writing to tracking session", err)

	err = m.Done(id, "id1")
	if err != nil {
		t.Fatal(err)
	}
}
