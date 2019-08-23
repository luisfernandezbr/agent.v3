package requests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/go-hclog"
)

func testOpts() (res Opts) {
	res.Logger = hclog.New(hclog.DefaultOptions)
	res.Client = http.DefaultClient
	return
}

func TestJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"a":1},{"a":2}]`)
	}))
	defer server.Close()

	ctx := context.Background()
	opts := testOpts()
	type obj struct {
		A int `json:"a"`
	}
	var res []obj
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = JSON(ctx, opts, req, &res)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []obj{{A: 1}, {A: 2}}, res)
}
