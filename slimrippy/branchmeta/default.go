package branchmeta

import (
	"context"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type Branch struct {
	Name   string
	Commit string
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
