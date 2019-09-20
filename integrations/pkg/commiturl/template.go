package commiturl

import (
	"strings"

	"github.com/pinpt/agent.next/integrations/pkg/commonrepo"
)

func CommitURLTemplate(repo commonrepo.Repo, repoURLPrefix string) string {
	return urlAppend(repoURLPrefix, repo.NameWithOwner) + "/commit/@@@sha@@@"
}

func BranchURLTemplate(repo commonrepo.Repo, repoURLPrefix string) string {
	return urlAppend(repoURLPrefix, repo.NameWithOwner) + "/tree/@@@branch@@@"
}

func urlAppend(p1, p2 string) string {
	return strings.TrimSuffix(p1, "/") + "/" + p2
}
