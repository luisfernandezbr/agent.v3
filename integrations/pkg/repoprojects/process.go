package repoprojects

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/cmd/cmdrunnorestarts/inconfig"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/pkg/ids2"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/go-common/v10/datamodel"
)

type ProcessOpts struct {
	Logger               hclog.Logger
	ProjectFn            ProjectFn
	ProjectLastProcessFn ProjectLastProcessFn
	Concurrency          int
	Projects             []RepoProject

	IntegrationType inconfig.IntegrationType
	CustomerID      string
	RefType         string

	Sender *objsender.Session
}

type ProjectFn func(ctx *ProjectCtx) error
type ProjectLastProcessFn func(ctx *ProjectCtx) (string, error)

type Process struct {
	opts ProcessOpts
}

func NewProcess(opts ProcessOpts) *Process {
	if opts.Logger == nil || (opts.ProjectFn == nil && opts.ProjectLastProcessFn == nil) || opts.Concurrency == 0 || opts.IntegrationType.String() == "unset" || opts.CustomerID == "" || opts.RefType == "" || opts.Sender == nil {
		panic("provide all args")
	}
	s := &Process{}
	s.opts = opts
	return s
}

func (s *Process) projectID(project RepoProject) string {
	ids := ids2.New(s.opts.CustomerID, s.opts.RefType)
	switch s.opts.IntegrationType {
	case inconfig.IntegrationTypeSourcecode:
		return ids.CodeRepo(project.GetID())
	case inconfig.IntegrationTypeWork:
		return ids.WorkProject(project.GetID())
	case inconfig.IntegrationTypeCalendar:
		return ids.CalendarCalendar(project.GetID())
	default:
		panic(fmt.Errorf("not supported IntegrationType: %v", s.opts.IntegrationType))
	}
}

const returnEarlyAfterNFailedProjects = 10

func (s *Process) Run() (allRes []rpcdef.ExportProject, rerr error) {
	if len(s.opts.Projects) == 0 {
		s.opts.Logger.Warn("no repos/projects found")
		return
	}

	wg := sync.WaitGroup{}

	var fatalErr error
	failedCount := 0
	var mu sync.Mutex

	projectsChan := projectsToChan(s.opts.Projects)

	for i := 0; i < s.opts.Concurrency; i++ {
		rerr := func(err error) {
			mu.Lock()
			if fatalErr == nil {
				fatalErr = err
			}
			mu.Unlock()
		}

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

				logger := s.opts.Logger.With("project_name", p.GetReadableID(), "project_ref_id", p.GetID())
				ctx := newProjectCtx(logger, p, s.opts.Sender)

				logger.Info("starting processing repo/project")
				var lastProcess string
				var projectErr error
				if s.opts.ProjectFn != nil {
					projectErr = s.opts.ProjectFn(ctx)
				} else {
					lastProcess, projectErr = s.opts.ProjectLastProcessFn(ctx)
				}
				logger.Info("completed processing repo/project", "err", projectErr)

				err := s.opts.Sender.IncProgress()
				if err != nil {
					rerr(err)
					return
				}

				p2 := rpcdef.ExportProject{}
				p2.ID = s.projectID(p)
				p2.RefID = p.GetID()
				p2.ReadableID = p.GetReadableID()
				if projectErr != nil {
					p2.Error = projectErr.Error()
				}

				mu.Lock()
				allRes = append(allRes, p2)
				if projectErr != nil {
					failedCount++
				}
				mu.Unlock()

				if projectErr != nil {
					err := ctx.rollback()
					if err != nil {
						rerr(err)
						return
					}
					return
				}
				if lastProcess != "" {
					err = ctx.doneLastProcess(lastProcess)
				} else {
					err = ctx.done()
				}
				if err != nil {
					rerr(err)
					return
				}
			}
		}()
	}
	wg.Wait()

	if fatalErr != nil {
		rerr = fatalErr
		return
	}

	if failedCount > 0 {
		s.opts.Logger.Error("Export failed on one or more repos/projects", "failed_count", failedCount)
	} else {
		s.opts.Logger.Info("Repo/project export completed without errors")
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

	senders     []*objsender.Session
	senderModel []string
	sendersMu   sync.Mutex
}

func newProjectCtx(logger hclog.Logger, project RepoProject, sender *objsender.Session) *ProjectCtx {
	s := &ProjectCtx{}
	s.Project = project
	s.Logger = logger
	s.sender = sender
	return s
}

func (s *ProjectCtx) Session(modelName datamodel.ModelNameType) (_ *objsender.Session, rerr error) {
	s.sendersMu.Lock()
	defer s.sendersMu.Unlock()

	sender, err := s.sender.Session(modelName.String(), s.Project.GetID(), s.Project.GetReadableID())
	if err != nil {
		rerr = err
		return
	}
	s.senders = append(s.senders, sender)
	s.senderModel = append(s.senderModel, modelName.String())
	return sender, nil
}

func (s *ProjectCtx) done() error {
	for i, sender := range s.senders {
		err := sender.Done()
		if err != nil {
			return fmt.Errorf("failed Done on sender model=%v %v", s.senderModel[i], err)
		}
	}
	return nil
}
func (s *ProjectCtx) doneLastProcess(token string) error {
	for i, sender := range s.senders {
		err := sender.DoneLastProcessed(token)
		if err != nil {
			return fmt.Errorf("failed DoneLastProcessed on sender model=%v %v", s.senderModel[i], err)
		}
	}
	return nil
}

func (s *ProjectCtx) rollback() error {
	for _, sender := range s.senders {
		err := sender.Rollback()
		if err != nil {
			return err
		}
	}
	return nil
}
