package api

import (
	"fmt"
	purl "net/url"
	"time"

	"github.com/pinpt/go-common/hash"
	"github.com/pinpt/integration-sdk/sourcecode"
)

type commitUser struct {
	Author struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"author"`
	Committer struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"committer"`
}

// FetchCommitUsers calls the commits api to get user information and returns a list of unique sourcecode.User
func (a *TFSAPI) FetchCommitUsers(repoid string, usermap map[string]*sourcecode.User, fromdate time.Time) error {

	url := fmt.Sprintf(`_apis/git/repositories/%s/commits`, purl.PathEscape(repoid))
	var res []commitUser
	if err := a.doRequest(url, nil, fromdate, &res); err != nil {
		return err
	}
	for _, user := range res {
		authoremail := user.Author.Email
		if authoremail != "" {
			authorname := user.Author.Name
			if usermap[authoremail] == nil {
				usermap[authoremail] = &sourcecode.User{
					Email:      &authoremail,
					Name:       user.Author.Name,
					RefType:    a.reftype,
					RefID:      hash.Values(authorname, authoremail),
					CustomerID: a.customerid,
				}
			}
		}
		committeremail := user.Committer.Email
		if committeremail != "" {
			committername := user.Committer.Name
			if authoremail != committeremail && usermap[committeremail] == nil {
				usermap[committeremail] = &sourcecode.User{
					Email:      &committeremail,
					Name:       user.Committer.Name,
					RefType:    a.reftype,
					RefID:      hash.Values(committername, authoremail),
					CustomerID: a.customerid,
				}
			}
		}
	}
	return nil
}
