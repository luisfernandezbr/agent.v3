package api

import (
	"net/url"
	"strings"
	"time"

	"github.com/pinpt/agent.next/pkg/commitusers"

	"github.com/pinpt/agent.next/pkg/date"
	"github.com/pinpt/agent.next/pkg/ids"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/agent"
)

func LastCommit(qc QueryContext, repo *agent.RepoResponseRepos) (lastCommit agent.RepoResponseReposLastCommit, err error) {
	qc.Logger.Debug("onboard repos request")

	objectPath := pstrings.JoinURL("repositories", repo.Name, "commits")

	params := url.Values{}
	params.Set("pagelen", "1")

	var rcommits []struct {
		HASH    string `json:"hash"`
		Message string `json:"message"`
		Author  struct {
			Raw  string `json:"raw"`
			User struct {
				DisplayName string `json:"display_name"`
				Links       struct {
					Avatar struct {
						Href string `json:"href"`
					} `json:"avatar"`
				} `json:"links"`
			} `json:"user"`
		} `json:"author"`
		Date time.Time `json:"date"`
	}

	_, err = qc.Request(objectPath, params, true, &rcommits)
	if err != nil {
		return
	}

	if len(rcommits) == 0 {
		return lastCommit, nil
	}

	lastCommitSource := rcommits[0]
	url, err := url.Parse(qc.BaseURL)
	if err != nil {
		return lastCommit, err
	}
	lastCommit = agent.RepoResponseReposLastCommit{
		Message:   lastCommitSource.Message,
		URL:       url.Scheme + "://" + strings.TrimPrefix(url.Hostname(), "api.") + "/" + repo.Name + "/commits/" + lastCommitSource.HASH,
		CommitSha: lastCommitSource.HASH,
		CommitID:  ids.CodeCommit(qc.CustomerID, qc.RefType, repo.RefID, lastCommitSource.HASH),
	}

	authorLastCommit := agent.RepoResponseReposLastCommitAuthor{}
	authorLastCommit.Name = lastCommitSource.Author.User.DisplayName
	_, authorLastCommit.Email = getNameAndEmail(lastCommitSource.Author.Raw)
	authorLastCommit.AvatarURL = lastCommitSource.Author.User.Links.Avatar.Href

	lastCommit.Author = authorLastCommit

	date.ConvertToModel(lastCommitSource.Date, &lastCommit.CreatedDate)

	return
}

func CommitUsersSourcecodePage(qc QueryContext, repo string, params url.Values) (page PageInfo, users []commitusers.CommitUser, err error) {
	qc.Logger.Debug("commit users request", "repo", repo)

	objectPath := pstrings.JoinURL("repositories", repo, "commits")

	params.Set("pagelen", "100")

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

	page, err = qc.Request(objectPath, params, true, &rcommits)
	if err != nil {
		return
	}

	for _, c := range rcommits {

		name := c.Author.User.DisplayName
		if name == "" {
			name, _ = getNameAndEmail(c.Author.Raw)
		}

		user := commitusers.CommitUser{}
		user.CustomerID = qc.CustomerID
		user.Name = name
		user.SourceID = c.Author.User.AccountID
		_, user.Email = getNameAndEmail(c.Author.Raw)

		users = append(users, user)
	}

	return
}

func getNameAndEmail(raw string) (string, string) {

	if raw == "" {
		return "", ""
	}

	index := strings.Index(raw, "<")

	return raw[:index-1], raw[index+1 : len(raw)-1]
}
