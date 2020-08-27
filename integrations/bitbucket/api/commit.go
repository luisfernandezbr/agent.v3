package api

import (
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/pkg/commitusers"

	pstrings "github.com/pinpt/go-common/v10/strings"
)

func CommitUsersSourcecodePage(
	qc QueryContext,
	logger hclog.Logger,
	repo string,
	defaultBranch string,
	params url.Values,
	stopOnUpdatedAt time.Time,
	nextPage NextPage) (np NextPage, users []commitusers.CommitUser, err error) {

	logger.Debug("commit users", "default_branch", defaultBranch, "inc_date", stopOnUpdatedAt, "params", params, "next", nextPage)

	objectPath := pstrings.JoinURL("repositories", repo, "commits", defaultBranch)

	var rcommits []struct {
		Author struct {
			Raw  string `json:"raw"`
			User struct {
				DisplayName string `json:"display_name"`
				AccountID   string `json:"account_id"`
			} `json:"user"`
		} `json:"author"`
		Date time.Time `json:"date"`
	}

	np, err = qc.Request(objectPath, params, true, &rcommits, nextPage)
	if err != nil {
		return
	}

	for _, c := range rcommits {
		if c.Date.Before(stopOnUpdatedAt) {
			np = ""
			return
		}
		name := c.Author.User.DisplayName
		if name == "" {
			name, _ = GetNameAndEmail(c.Author.Raw)
		}

		user := commitusers.CommitUser{}
		user.CustomerID = qc.CustomerID
		user.Name = name
		user.SourceID = c.Author.User.AccountID
		_, user.Email = GetNameAndEmail(c.Author.Raw)
		if user.Email == "" {
			continue
		}

		users = append(users, user)
	}

	return
}

func GetNameAndEmail(raw string) (name string, email string) {
	if raw == "" {
		return
	}

	index := strings.Index(raw, "<")

	if index == 0 {
		name = ""
		email = getSubstring(raw, index+1, len(raw)-1)

		return
	}

	name = getSubstring(raw, 0, index-1)
	email = getSubstring(raw, index+1, len(raw)-1)

	return
}

func getSubstring(str string, ind1, ind2 int) (res string) {
	if !validateIndex(str, ind1) {
		return
	}
	if !validateIndex(str, ind2) {
		return
	}
	if ind2 < ind1 {
		return
	}
	return str[ind1:ind2]
}

func validateIndex(str string, index int) bool {
	return index <= len(str) && index > -1
}
