package branchmeta

import (
	"context"
	"strings"

	"github.com/pinpt/agent/slimrippy/internal/repoutil"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type Branch struct {
	Name   string
	Commit string
}

func GetAll(ctx context.Context, repoDir string, includeDefault bool) (res []Branch, rerr error) {
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		rerr = err
		return
	}

	defaultHeadRef, err := repo.Reference(plumbing.HEAD, true)
	if err != nil {
		rerr = err
		return
	}
	defaultHead := defaultHeadRef.Name().String()
	branches, err := repoutil.RepoAllBranchIter(repo)
	if err != nil {
		rerr = err
		return
	}
	for _, ref := range branches {
		if ref.Hash().IsZero() {
			continue
		}
		b := Branch{}
		b.Name = strings.TrimPrefix(ref.Name().Short(), "origin/")
		if !includeDefault && ref.Name().String() == defaultHead {
			continue
		}
		b.Commit = ref.Hash().String()
		res = append(res, b)
	}
	return
}

func startsWith(b string, prefix string) bool {
	if len(prefix) > len(b) {
		return false
	}
	return string(b[:len(prefix)]) == prefix
}

func GetDefault(ctx context.Context, repoDir string) (res Branch, rerr error) {
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		rerr = err
		return
	}
	defaultHeadRef, err := repo.Reference(plumbing.HEAD, true)
	if err != nil {
		rerr = err
		return
	}
	if defaultHeadRef.Hash().IsZero() {
		panic("no default")
	}
	res.Name = defaultHeadRef.Name().Short()
	res.Commit = defaultHeadRef.Hash().String()
	return
}
