package mutate

import (
	"net/http"

	"github.com/pinpt/agent/pkg/requests2"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/integration-sdk/agent"
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

func UnmarshalAction(fn string) (v agent.IntegrationMutationRequestAction) {

	/*
		Below example doesn't work due to bug in schemagen
		var action agent.IntegrationMutationRequestAction
		err = action.UnmarshalJSON([]byte("ISSUE_SET_TITLE"))
		if err != nil {
			panic(err)
		}
		//fmt.Println(action)
	*/
	switch fn {
	case "ISSUE_ADD_COMMENT":
		v = 0
	case "ISSUE_SET_TITLE":
		v = 1
	case "ISSUE_SET_STATUS":
		v = 2
	case "ISSUE_SET_PRIORITY":
		v = 3
	case "ISSUE_SET_ASSIGNEE":
		v = 4
	case "ISSUE_GET_TRANSITIONS":
		v = 5
	case "PR_SET_TITLE":
		v = 6
	case "PR_SET_DESCRIPTION":
		v = 7
	default:
		panic("unsupported action: " + fn)
	}
	return
}

func ResultFromError(err error) (res rpcdef.MutateResult) {
	var e requests2.StatusCodeError
	if errors.As(err, &e) && e.Got == http.StatusNotFound {
		res.ErrorCode = ErrNotFound
	}
	res.Error = err.Error()
	return res
}
