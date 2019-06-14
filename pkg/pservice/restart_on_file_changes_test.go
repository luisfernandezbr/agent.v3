package pservice

/*
import (
	"context"
	"io/ioutil"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testRun struct {
	t  *testing.T
	ev []string
	mu sync.Mutex
}

func (s *testRun) Run(ctx context.Context) error {
	s.t.Log("tr: start")
	s.mu.Lock()
	s.ev = append(s.ev, "start")
	s.mu.Unlock()

	<-ctx.Done()

	s.t.Log("tr: stop")
	s.mu.Lock()
	s.ev = append(s.ev, "stop")
	s.mu.Unlock()

	return nil
}

func (s *testRun) Events() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ev
}

func (s *testRun) EventsClear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ev = nil
}

func TestRestartOnFileChanges(t *testing.T) {
	ts := &testRun{t: t}
	logger := &testLogger{t: t}
	tf := newTempFs()
	defer tf.Remove()

	run := RestartOnFileChanges(logger, ts.Run, tf.File)

	changeFile := func() {
		c := strconv.FormatInt(rand.Int63(), 10)
		err := ioutil.WriteFile(tf.File, []byte(c), 0666)
		if err != nil {
			panic(err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Run("basic", func(t *testing.T) {
		ts.EventsClear()
		assert := assert.New(t)
		for i := 0; i < 3; i++ {
			done, cancel := AsyncRunBg(run)
			cancel()
			err := <-done
			assert.NoError(err)
			assert.Equal([]string{"start", "stop"}, ts.Events())
			ts.EventsClear()
		}
	})

	t.Run("not-started", func(t *testing.T) {
		ts.EventsClear()
		assert := assert.New(t)
		// not started, editing file should not start
		changeFile()
		assert.Empty(ts.Events(), "not started, editing file should not start")
	})

	t.Run("started", func(t *testing.T) {
		ts.EventsClear()
		assert := assert.New(t)

		// started, editing file should restart
		done, cancel := AsyncRunBg(run)
		time.Sleep(50 * time.Millisecond)
		ts.EventsClear()

		changeFile()
		assert.Equal([]string{"stop", "start"}, ts.Events(), "started, editing file should restart")

		cancel()
		err := <-done
		assert.NoError(err)
	})
}
*/
