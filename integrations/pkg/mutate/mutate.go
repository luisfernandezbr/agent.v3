package mutate

import (
	"net/http"

	"github.com/pinpt/agent/pkg/requests"
	"github.com/pinpt/agent/rpcdef"
	"golang.org/x/exp/errors"
)

type IssueTransition struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Fields []IssueTransitionField `json:"fields"`
}

type IssueTransitionField struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	AllowedValues []AllowedValue `json:"allowed_values"`
	Required      bool           `json:"required"`
}

type AllowedValue struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

const ErrNotFound = "not_found"

func ResultFromError(err error) (res rpcdef.MutateResult) {
	var e requests.StatusCodeError
	if errors.As(err, &e) && e.Got == http.StatusNotFound {
		res.ErrorCode = ErrNotFound
	}
	res.Error = err.Error()
	return res
}
