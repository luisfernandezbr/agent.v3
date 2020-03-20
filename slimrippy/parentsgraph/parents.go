package parentsgraph

import (
	"sort"

	"github.com/pinpt/agent/slimrippy/pkg/repoutil"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type Graph struct {
	opts     Opts
	Parents  map[string][]string
	Children map[string][]string
}

type Opts struct {
	RepoDir     string
	AllBranches bool
	UseOrigin   bool
	//Logger      logger.Logger
}

func New(opts Opts) *Graph {
	s := &Graph{}
	s.opts = opts
	return s
}

func NewFromMap(parents map[string][]string) *Graph {
	s := &Graph{}
	s.Parents = parents
	s.childrenFromParents()
	return s
}

func (s *Graph) Read() error {
	//start := time.Now()
	//s.opts.Logger.Info("parentsgraph: starting reading")
	//defer func() {
	//s.opts.Logger.Info("parentsgraph: completed reading", "d", time.Since(start))
	//}()
	err := s.retrieveParents()
	if err != nil {
		return err
	}
	s.childrenFromParents()
	return nil
}

func (s *Graph) childrenFromParents() {
	s.Children = map[string][]string{}
	for commit, parents := range s.Parents {
		if _, ok := s.Children[commit]; !ok {
			// make sure that even if commit does not have any children we have a map key for it
			s.Children[commit] = nil
		}
		for _, p := range parents {
			s.Children[p] = append(s.Children[p], commit)
		}
	}
	for _, data := range s.Children {
		sort.Strings(data)
	}
}

func (s *Graph) retrieveParents() (rerr error) {
	s.Parents = map[string][]string{}
	repo, err := git.PlainOpen(s.opts.RepoDir)
	if err != nil {
		rerr = err
		return
	}
	return repoutil.RepoAllCommits(repo, s.opts.UseOrigin, nil, func(c *object.Commit) error {
		var parents []string
		for _, p := range c.ParentHashes {
			parents = append(parents, p.String())
		}
		s.Parents[c.Hash.String()] = parents
		return nil
	})
}
