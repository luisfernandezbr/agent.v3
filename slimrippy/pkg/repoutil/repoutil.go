package repoutil

import (
	"fmt"
	"io"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

func RepoAllBranchIter(repo *git.Repository) (res []*plumbing.Reference, rerr error) {
	refIter, err := repo.Storer.IterReferences()
	if err != nil {
		rerr = err
		return
	}
	hasRemotes := false
	refIter.ForEach(func(r *plumbing.Reference) error {
		name := string(r.Name())
		if startsWith(name, "refs/remotes/origin/") {
			hasRemotes = true
		}
		return nil
	})
	prefix := ""
	if hasRemotes {
		prefix = "refs/remotes/origin/"
	} else {
		prefix = "refs/heads/"
	}
	refIter, err = repo.Storer.IterReferences()
	if err != nil {
		rerr = err
		return
	}
	refIter.ForEach(func(r *plumbing.Reference) error {
		name := string(r.Name())
		if name == prefix+"HEAD" {
			return nil
		}
		if startsWith(name, prefix) {
			res = append(res, r)
		}
		return nil
	})
	return
}

func startsWith(b string, prefix string) bool {
	if len(prefix) > len(b) {
		return false
	}
	return string(b[:len(prefix)]) == prefix
}

func RepoAllCommits(repo *git.Repository, seenExternal map[plumbing.Hash]bool, cb func(*object.Commit) error) (rerr error) {
	ret := func(err error) {
		rerr = fmt.Errorf("RepoAllCommits failed: %v", err)
	}

	// The following does not work, missing some commits running on kubernetes/kops repo.
	// Looks like a bug in go-git.v4.
	//	iter, err := repo.Log(&git.LogOptions{ All:   true, })
	refs, err := RepoAllBranchIter(repo)
	if err != nil {
		ret(err)
		return
	}
	seen := map[plumbing.Hash]bool{}
	for k := range seenExternal {
		seen[k] = true
	}
	iter := func(c *object.Commit) object.CommitIter {
		return object.NewCommitPreorderIter(c, seen, nil)
	}
	for _, ref := range refs {
		commit, err := repo.CommitObject(ref.Hash())
		if err != nil {
			ret(err)
			return
		}
		iter := iter(commit)
		err = iter.ForEach(func(c1 *object.Commit) error {
			seen[c1.Hash] = true
			return cb(c1)
		})
		if err != nil && err != io.EOF {
			ret(err)
		}
	}
	return nil
}
