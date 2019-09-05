package api

import (
	"fmt"
	purl "net/url"
	"time"
)

type RawCommitUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}
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
func (a *TFSAPI) FetchCommitUsers(repoid string, usermap map[string]*RawCommitUser, fromdate time.Time) error {

	url := fmt.Sprintf(`_apis/git/repositories/%s/commits`, purl.PathEscape(repoid))
	var res []commitUser
	if err := a.doRequest(url, nil, fromdate, &res); err != nil {
		return err
	}
	for _, user := range res {
		authorname := user.Author.Name
		authoremail := user.Author.Email
		if authorname != "" && authoremail != "" {
			if usermap[authorname] == nil {
				usermap[authorname] = &RawCommitUser{
					Name:  authorname,
					Email: authoremail,
				}
			}
		}
		committername := user.Committer.Name
		committeremail := user.Committer.Email
		if committername != "" && committeremail != "" {
			if authorname != committername && usermap[committername] == nil {
				usermap[committername] = &RawCommitUser{
					Name:  committername,
					Email: user.Committer.Email,
				}
			}
		}
	}
	return nil
}
