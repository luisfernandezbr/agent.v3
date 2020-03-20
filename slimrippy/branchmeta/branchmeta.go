package branchmeta

import (
	"context"
	"strings"

	"github.com/pinpt/agent/slimrippy/pkg/repoutil"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type Opts struct {
	//Logger         logger.Logger
	RepoDir        string
	UseOrigin      bool
	IncludeDefault bool
}

type BranchWithCommitTime struct {
	Name   string
	Commit string
	//CommitCommitterTime time.Time
}

func Get(ctx context.Context, opts Opts) (res []BranchWithCommitTime, rerr error) {
	repo, err := git.PlainOpen(opts.RepoDir)
	if err != nil {
		rerr = err
		return
	}

	//prefix := "refs/remotes/origin/"
	//if !opts.UseOrigin {
	//	prefix = "refs/heads/"
	//}
	defaultHeadRef, err := repo.Reference(plumbing.HEAD, true)
	if err != nil {
		rerr = err
		return
	}
	defaultHead := defaultHeadRef.Name().String()
	branches, err := repoutil.RepoAllBranchIter(repo, opts.UseOrigin)
	//iter, err := repo.Storer.IterReferences()
	if err != nil {
		rerr = err
		return
	}
	for _, ref := range branches {
		if ref.Hash().IsZero() {
			continue
		}
		b := BranchWithCommitTime{}
		b.Name = strings.TrimPrefix(ref.Name().Short(), "origin/")
		if !opts.IncludeDefault && ref.Name().String() == defaultHead {
			continue
		}
		b.Commit = ref.Hash().String()
		/*
			c, err := repo.CommitObject(ref.Hash())
			if err != nil {
				return err
			}
			b.CommitCommitterTime = c.Committer.When*/
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
