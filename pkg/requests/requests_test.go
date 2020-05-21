package requests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/go-hclog"
)

func testRequests() Requests {
	return New(hclog.New(hclog.DefaultOptions), http.DefaultClient)
}

func TestJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok {
			t.Error("basic auth not passed")
		} else if u != "u" || p != "p" {
			t.Error("basic auth invalid values")
		}
		fmt.Fprint(w, `[{"a":1},{"a":2}]`)
	}))
	defer server.Close()

	r := testRequests()
	type obj struct {
		A int `json:"a"`
	}
	var res []obj
	req := Request{URL: server.URL, BasicAuthUser: "u", BasicAuthPassword: "p"}

	_, err := r.JSON(req, &res)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []obj{{A: 1}, {A: 2}}, res)
}
