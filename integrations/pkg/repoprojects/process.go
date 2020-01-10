package repoprojects

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/pkg/ids2"
	"github.com/pinpt/agent/pkg/integrationid"
	"github.com/pinpt/agent/rpcdef"
)

type ProcessOpts struct {
	Logger      hclog.Logger
	ProjectFn   ProjectFn
	Concurrency int
	Projects    []RepoProject

	IntegrationType integrationid.Type
	CustomerID      string
	RefType         string

	Sender *objsender.Session
}

type ProjectFn func(ctx *ProjectCtx) error

type Process struct {
	opts ProcessOpts
}

func NewProcess(opts ProcessOpts) *Process {
	if opts.Logger == nil || opts.ProjectFn == nil || opts.Concurrency == 0 || len(opts.Projects) == 0 || opts.IntegrationType == "" || opts.CustomerID == "" || opts.RefType == "" || opts.Sender == nil {
		panic("provide all args")
	}

	s := &Process{}
	s.opts = opts
	return s
}

func (s *Process) projectID(project RepoProject) string {
	ids := ids2.New(s.opts.CustomerID, s.opts.RefType)
	switch s.opts.IntegrationType {
	case integrationid.TypeSourcecode:
		return ids.CodeRepo(project.GetID())
	case integrationid.TypeWork:
		return ids.WorkProject(project.GetID())
	default:
		panic(fmt.Errorf("not supported IntegrationType: %v", s.opts.IntegrationType))
	}
}

const returnEarlyAfterNFailedProjects = 10

func (s *Process) Run() (allRes []rpcdef.ExportProject, rerr error) {
	wg := sync.WaitGroup{}

	var fatalErr error
	failedCount := 0
	var mu sync.Mutex

	projectsChan := projectsToChan(s.opts.Projects)

	for i := 0; i < s.opts.Concurrency; i++ {
		/*
			rerr := func(err error) {
				mu.Lock()
				if fatalErr == nil {
					fatalErr = err
				}
				mu.Unlock()
			}*/

		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range projectsChan {
				mu.Lock()
				stop := fatalErr != nil || failedCount > returnEarlyAfterNFailedProjects
				mu.Unlock()
				if stop {
					return
				}

				ctx := newProjectCtx(s.opts.Logger, p, s.opts.Sender)
				err := s.opts.ProjectFn(ctx)

				p2 := rpcdef.ExportProject{}
				p2.ID = s.projectID(p)
				p2.RefID = p.GetID()
				p2.ReadableID = p.GetReadableID()
				if err != nil {
					p2.Error = err.Error()
				}

				mu.Lock()
				allRes = append(allRes, p2)
				if err != nil {
					failedCount++
				}
				mu.Unlock()

				if err != nil {
					panic("TODO: this is not implemented, needs changes in last processed handling")
					//err := ctx.DoneWithoutUpdatingLastProcessed()
					//if err != nil {
					//	rerr(err)
					//	return
					//}
				}

				//err = ctx.Done()
				//if err != nil {
				//	rerr(err)
				//	return
				//}
			}
		}()
	}
	wg.Wait()

	if fatalErr != nil {
		rerr = fatalErr
		return
	}

	return
}

func projectsToChan(projects []RepoProject) chan RepoProject {
	res := make(chan RepoProject)
	go func() {
		for _, p := range projects {
			res <- p
		}
		close(res)
	}()
	return res
}

type ProjectCtx struct {
	Project RepoProject
	Logger  hclog.Logger

	sender *objsender.Session

	senders   []*objsender.Session
	sendersMu sync.Mutex
}

func newProjectCtx(logger hclog.Logger, project RepoProject, sender *objsender.Session) *ProjectCtx {
	s := &ProjectCtx{}
	s.Project = project
	s.Logger = logger
	s.sender = sender
	return s
}

/*
func (s *ProjectCtx) Session(modelName datamodel.ModelNameType, id string, name string) (_ *objsender.Session, rerr error) {
	s.sendersMu.Lock()
	defer s.sendersMu.Unlock()

	sender, err := s.sender.Session(modelName.String(), id, name)
	if err != nil {
		rerr = err
		return
	}
	s.senders = append(s.senders, sender)
	return sender, nil
}

func (s *ProjectCtx) Done() error {
	for _, sender := range s.senders {
		err := sender.Done()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *ProjectCtx) DoneWithoutUpdatingLastProcessed() error {
	for _, sender := range s.senders {
		err := sender.DoneWithoutUpdatingLastProcessed()
		if err != nil {
			return err
		}
	}
	return nil
}
*/
