package commits

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/pinpt/agent/slimrippy/pkg/repoutil"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type Opts struct {
	State   State
	RepoDir string
}

type State struct {
	CommitsSeen CommitsSeen
}

type CommitsSeen map[Hash]bool

type Hash [20]byte

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

func (s CommitsSeen) MarshalJSON() ([]byte, error) {
	res := []string{}
	for c := range s {
		res = append(res, c.String())
	}
	return json.Marshal(res)
}

func (s *CommitsSeen) UnmarshalJSON(b []byte) error {
	m := map[Hash]bool{}
	var commits []string
	if err := json.Unmarshal(b, &commits); err != nil {
		return err
	}
	for _, c := range commits {
		h := plumbing.NewHash(c)
		m[Hash(h)] = true
	}
	*s = m
	return nil
}

type UserAction struct {
	Email string
	Name  string
	Date  time.Time
}

type Commit struct {
	SHA       string
	Authored  UserAction
	Committed UserAction
	Message   string
}

func Commits(ctx context.Context, opts Opts, res chan *object.Commit) (_ State, rerr error) {
	defer close(res)
	if opts.RepoDir == "" {
		rerr = errors.New("RepoDir not set")
		return
	}
	repo, err := git.PlainOpen(opts.RepoDir)
	if err != nil {
		rerr = err
		return
	}
	if opts.State.CommitsSeen == nil {
		opts.State.CommitsSeen = map[Hash]bool{}
	}
	commitSeen := opts.State.CommitsSeen
	commitSeen2 := map[plumbing.Hash]bool{}
	for c := range commitSeen {
		commitSeen2[plumbing.Hash(c)] = true
	}
	repoutil.RepoAllCommits(repo, commitSeen2, func(c *object.Commit) error {
		h := Hash(c.Hash)
		commitSeen[h] = true
		res <- c
		return nil
	})
	return opts.State, nil
}

func Convert(c1 *object.Commit) Commit {
	c2 := Commit{}
	c2.SHA = c1.Hash.String()
	c2.Authored.Email = c1.Author.Email
	c2.Authored.Name = c1.Author.Name
	c2.Authored.Date = c1.Author.When
	c2.Committed.Email = c1.Committer.Email
	c2.Committed.Name = c1.Committer.Name
	c2.Committed.Date = c1.Committer.When
	c2.Message = strings.TrimSpace(c1.Message)
	return c2
}

func CommitsSlice(ctx context.Context, opts Opts) (res []Commit, _ State, rerr error) {
	ch := make(chan *object.Commit)
	done := make(chan bool)
	go func() {
		for c := range ch {
			res = append(res, Convert(c))
		}
		done <- true
	}()
	state, err := Commits(ctx, opts, ch)
	if err != nil {
		rerr = err
		return
	}
	<-done
	return res, state, nil
}
