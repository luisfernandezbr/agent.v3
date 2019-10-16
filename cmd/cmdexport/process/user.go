package process

import (
	"errors"
	"sync"

	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/integration-sdk/sourcecode"
)

type CommitUsers struct {
	data map[string]bool
	mu   sync.Mutex
}

func NewCommitUsers() *CommitUsers {
	s := &CommitUsers{}
	s.data = map[string]bool{}
	return s
}

func (s *CommitUsers) Transform(data map[string]interface{}) (_ map[string]interface{}, _ error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	customerID, _ := data["customer_id"].(string)
	if customerID == "" {
		return nil, errors.New("customer_id is required")
	}
	email, _ := data["email"].(string)
	if email == "" {
		return nil, errors.New("email is required")
	}
	name, _ := data["name"].(string)
	if name == "" {
		return nil, errors.New("name is required")
	}
	sourceID, _ := data["source_id"].(string)

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
