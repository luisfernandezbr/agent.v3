package url

import (
	"net/url"
	"strings"
)

type customCondition func(url *url.URL)

func GetRepoURL(repoURLPrefix string, user *url.Userinfo, nameWithOwner string, cc customCondition) (string, error) {

	if strings.Contains(repoURLPrefix, "api.bitbucket.org") {
		repoURLPrefix = strings.Replace(repoURLPrefix, "api.", "", -1)
	}

	u, err := url.Parse(repoURLPrefix)
	if err != nil {
		return "", err
	}
	u.User = user
	if nameWithOwner != "" {
		u.Path = nameWithOwner
	}
	if cc != nil {
		cc(u)
	}
	return u.String(), nil
}
