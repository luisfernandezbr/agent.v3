package slimrippy

import (
	"context"
	"sync"
	"time"

	"github.com/pinpt/agent/slimrippy/internal/branchmeta"

	"github.com/pinpt/agent/slimrippy/internal/branches"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/slimrippy/internal/commits"
	"github.com/pinpt/agent/slimrippy/internal/parentsgraph"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type Branch = branches.Branch
type Commit = commits.Commit

type BranchLastCommit = branchmeta.Branch

func GetBranchesWithLastCommit(ctx context.Context, repoDir string) ([]BranchLastCommit, error) {
	// internal interface has optional bool to skip default branch
	// don't expose it to exportrepo
	return branchmeta.GetAll(ctx, repoDir, true)
}

type State struct {
	Commits commits.State
	Parents parentsgraph.State
}

type Opts struct {
	Logger          hclog.Logger
	RepoDir         string
	State           State
	PullRequestSHAs []string

	CommitCallback func(commits.Commit) error
	BranchCallback func(branches.Branch) error
}

func CommitsAndBranches(ctx context.Context, opts Opts) (_ State, rerr error) {
	state := opts.State
	logger := opts.Logger.Named("slimrippy")

	started := time.Now()
	defer func() {
		logger.Debug("commitsAndBranches done", "duration", time.Since(started))
	}()

	commitsForParents := make(chan *object.Commit)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		started := time.Now()
		defer func() {
			logger.Debug("commits done", "duration", time.Since(started))
		}()
		commitsChan := make(chan *object.Commit)
		done := make(chan bool)
		go func() {
			for c := range commitsChan {
				commitsForParents <- c
				if opts.CommitCallback != nil {
					err := opts.CommitCallback(commits.Convert(c))
					if err != nil {
						panic(err)
					}
				}
			}
			close(commitsForParents)
			done <- true
		}()
		cOpts := commits.Opts{}
		cOpts.State = state.Commits
		cOpts.RepoDir = opts.RepoDir
		cState, err := commits.Commits(ctx, cOpts, commitsChan)
		<-done
		if err != nil {
			panic(err)
		}
		state.Commits = cState
	}()

	var graph *parentsgraph.Graph
	wg.Add(1)
	go func() {
		defer wg.Done()
		started := time.Now()
		defer func() {
			logger.Debug("parents done", "duration", time.Since(started))
		}()
		popts := parentsgraph.Opts{}
		popts.State = state.Parents
		popts.Commits = commitsForParents
		g, s := parentsgraph.New(popts)
		graph = g
		state.Parents = s
	}()

	wg.Wait()

	{
		started := time.Now()
		defer func() {
			logger.Debug("branches done", "duration", time.Since(started))
		}()
		res := make(chan branches.Branch)
		done := make(chan bool)
		go func() {
			for b := range res {
				err := opts.BranchCallback(b)
				if err != nil {
					panic(err)
				}
			}
			done <- true
		}()
		bopts := branches.Opts{}
		bopts.IncludeDefaultBranch = true
		bopts.PullRequestSHAs = opts.PullRequestSHAs
		bopts.Logger = opts.Logger
		bopts.CommitGraph = graph
		bopts.RepoDir = opts.RepoDir
		b := branches.New(bopts)
		err := b.Run(ctx, res)
		<-done
		if err != nil {
			rerr = err
			return
		}
	}

	return state, nil
}
