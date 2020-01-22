package fsqueue

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/hashicorp/go-hclog"

)


func testLogger() hclog.Logger {
	return hclog.New(hclog.DefaultOptions)
}
func TestQueueRunCancel(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "db")
	q, _, err := New(testLogger(), file)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exited := make(chan bool)
	go func() {
		err := q.Run(ctx)
		if err != nil {
			t.Fatal(err)
		}
		exited <- true
	}()

	cancel()
	<-exited
}

func TestQueuePassData(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "db")
	q, forward, err := New(testLogger(), file)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exited := make(chan bool)
	go func() {
		err := q.Run(ctx)
		if err != nil {
			t.Fatal(err)
		}
		exited <- true
	}()

	fmt.Println("sending data 1")
	q.Input <- Data{"k1": "v1"}
	fmt.Println("sent data 1")
	q.Input <- Data{"k2": "v2"}
	fmt.Println("sent data 2")

	assert := assert.New(t)

	req := <-forward
	assert.Equal(Data{"k1": "v1"}, req.Data)
	req.Done <- struct{}{}
	req = <-forward
	assert.Equal(Data{"k2": "v2"}, req.Data)
	req.Done <- struct{}{}

	cancel()
	<-exited
}

func TestQueueStoreOnFS(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "db")

	assert := assert.New(t)

	runQueue := func(cb func(q *Queue, f chan Request)) {
		q, forward, err := New(testLogger(), file)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		exited := make(chan bool)
		go func() {
			err := q.Run(ctx)
			if err != nil {
				t.Fatal(err)
			}
			exited <- true
		}()

		cb(q, forward)

		cancel()
		<-exited
	}

	runQueue(func(q *Queue, forward chan Request) {
		q.Input <- Data{"k1": "v1"}
		q.Input <- Data{"k2": "v2"}

		req := <-forward
		assert.Equal(Data{"k1": "v1"}, req.Data)
		req.Done <- struct{}{}
		req = <-forward
		assert.Equal(Data{"k2": "v2"}, req.Data)
		// done is not called so when starting again it should be retrieved from fs and processed
		//req.Done <- struct{}{}

	})

	// dataDone is executed async, we don't know when it's done for sure
	time.Sleep(10 * time.Millisecond)

	runQueue(func(q *Queue, forward chan Request) {
		fmt.Println("run 2: waiting for data from fs")
		req := <-forward
		fmt.Println("run 2: got data from fs")
		assert.Equal(Data{"k2": "v2"}, req.Data)
		req.Done <- struct{}{}
	})
}
