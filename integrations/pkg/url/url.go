package url

import (
	"net/url"
	"strings"
)

func GetRepoURL(repoURLPrefix string, user *url.Userinfo, nameWithOwner string) (string, error) {

	if strings.Contains(repoURLPrefix, "api.bitbucket.org") {
		repoURLPrefix = strings.Replace(repoURLPrefix, "api.", "", -1)
	}

	u, err := url.Parse(repoURLPrefix)
	if err != nil {
		return "", err
	}
	u.User = user
	u.Path = nameWithOwner
	return u.String(), nil
}
