package process

import (
	"errors"
	"strings"
	"sync"

	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/integration-sdk/sourcecode"
	"github.com/hashicorp/go-hclog"
)

type CommitUsers struct {
	data map[string]bool
	mu   sync.Mutex
	logger hclog.Logger
}

func NewCommitUsers(logger hclog.Logger) *CommitUsers {
	s := &CommitUsers{}
	s.data = map[string]bool{}
	s.logger = logger
	return s
}

func (s *CommitUsers) Transform(data map[string]interface{}) (_ map[string]interface{}, _ error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	customerID, _ := data["customer_id"].(string)
	if customerID == "" {
		return nil, errors.New("customer_id is required")
	}

	name, _ := data["name"].(string)
	if name == "" {
		return nil, errors.New("name is required")
	}
	sourceID, _ := data["source_id"].(string)

	email, _ := data["email"].(string)
	if email == "" {
		s.logger.Warn("email is required","name",name)
	}

	// always convert email to lowercase
	email = strings.ToLower(email)

	// We only send the first name encountered. For this reason name is not present in hash.
	// TODO: maybe support multiple names, needs design discussion about pipeline
	key := email + "@@@" + sourceID

	if s.data[key] {
		// was already added
		return nil, nil
	}
	s.data[key] = true

	obj := sourcecode.User{}

	obj.CustomerID = customerID
	obj.RefType = "git"

	emailRef := hash.Values(customerID, email)
	obj.RefID = emailRef

	if sourceID == "" {
		// unlinked
		obj.ID = hash.Values("User", obj.CustomerID, email, "git")
	} else {
		obj.ID = hash.Values("User", obj.CustomerID, email, "git", sourceID)
	}

	obj.Email = &email
	obj.Name = name
	if sourceID != "" {
		obj.AssociatedRefID = &sourceID
	}

	res := obj.ToMap()

	return res, nil
}
