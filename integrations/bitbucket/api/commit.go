package api

import (
	"net/url"
	"regexp"
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
		URL:       url.Scheme + "://" + GetDomain(url.Hostname()) + "/" + repo.Name + "/commits/" + lastCommitSource.HASH,
		CommitSha: lastCommitSource.HASH,
		CommitID:  ids.CodeCommit(qc.CustomerID, qc.RefType, repo.RefID, lastCommitSource.HASH),
	}

	authorLastCommit := agent.RepoResponseReposLastCommitAuthor{
		Name:      lastCommitSource.Author.User.DisplayName,
		Email:     getEmailFromRaw(lastCommitSource.Author.Raw),
		AvatarURL: lastCommitSource.Author.User.Links.Avatar.Href,
	}

	lastCommit.Author = authorLastCommit

	date.ConvertToModel(lastCommitSource.Date, &lastCommit.CreatedDate)

	return
}

func GetDomain(hostname string) string {

	sub := strings.Split(hostname, ".")

	return strings.Join(sub[len(sub)-2:], ".")
}

func getEmailFromRaw(raw string) string {
	var re = regexp.MustCompile(`(?m)<(.*)>`)

	for _, match := range re.FindAllStringSubmatch(raw, -1) {
		return match[1]
	}

	return ""
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
			name = getUsernameFromRaw(c.Author.Raw)
		}

		user := commitusers.CommitUser{
			CustomerID: qc.CustomerID,
			Name:       name,
			Email:      getEmailFromRaw(c.Author.Raw),
			SourceID:   c.Author.User.AccountID,
		}

		users = append(users, user)
	}

	return
}

func getUsernameFromRaw(raw string) string {
	index := strings.Index(raw, "<")

	return raw[:index]
}
