package parentsgraph

import (
	"sort"

	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type Graph struct {
	opts     Opts
	Parents  map[string][]string
	Children map[string][]string
}

type Opts struct {
	//RepoDir     string
	//AllBranches bool
	State   State
	Commits chan *object.Commit
	//Logger      logger.Logger
}

type State struct {
	Parents map[string][]string
}

func New(opts Opts) (*Graph, State) {
	s := &Graph{}
	s.opts = opts
	if len(opts.State.Parents) != 0 {
		s.Parents = opts.State.Parents
	} else {
		s.Parents = map[string][]string{}
	}
	for c := range opts.Commits {
		var parents []string
		for _, p := range c.ParentHashes {
			parents = append(parents, p.String())
		}
		s.Parents[c.Hash.String()] = parents
	}
	s.childrenFromParents()
	return s, State{Parents: s.Parents}
}

func NewFromMap(parents map[string][]string) *Graph {
	s := &Graph{}
	s.Parents = parents
	s.childrenFromParents()
	return s
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
