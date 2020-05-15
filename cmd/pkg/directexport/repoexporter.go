package directexport

import (
	"context"
	"sync"

	"github.com/pinpt/agent/pkg/fsconf"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/cmd/cmdexport/process"
	"github.com/pinpt/agent/cmd/cmdintegration"
	"github.com/pinpt/agent/pkg/expsessions"
	"github.com/pinpt/agent/pkg/gitclone"
	"github.com/pinpt/agent/pkg/jsonstore"
	"github.com/pinpt/agent/rpcdef"
	"github.com/pinpt/agent/slimrippy/exportrepo"
)

type RepoExporterOpts struct {
	Logger        hclog.Logger
	AgentConfig   cmdintegration.AgentConfig
	LastProcessed *jsonstore.Store
	Locs          fsconf.Locs
}

type RepoExporter struct {
	opts      RepoExporterOpts
	logger    hclog.Logger
	repoFetch chan rpcdef.GitRepoFetch
}

func NewRepoExporter(opts RepoExporterOpts) *RepoExporter {
	return &RepoExporter{
		opts:      opts,
		repoFetch: make(chan rpcdef.GitRepoFetch, 1000),
		logger:    opts.Logger.Named("repo-exporter"),
	}
}

func (s RepoExporter) ExportGitRepo(fetch rpcdef.GitRepoFetch) error {
	s.repoFetch <- fetch
	return nil
}

type RepoExporterRes struct {
	Data rpcdef.MutatedObjects
	Err  error
}

func (s *RepoExporter) Done() {
	close(s.repoFetch)
}

// easier to return struct here to sync we call this async and want result back in chan
func (s RepoExporter) Run() (res RepoExporterRes) {
	s.logger.Debug("running repo exporter")

	writer := NewObjectsWriter()
	commitUsers := process.NewCommitUsers()

	for fetch := range s.repoFetch {
		access := gitclone.AccessDetails{}
		access.URL = fetch.URL

		opts := exportrepo.Opts{
			Logger:     s.logger,
			CustomerID: s.opts.AgentConfig.CustomerID,
			RepoID:     fetch.RepoID,
			UniqueName: fetch.UniqueName,
			RefType:    fetch.RefType,

			LastProcessed: s.opts.LastProcessed,
			RepoAccess:    access,

			CommitURLTemplate: fetch.CommitURLTemplate,
			BranchURLTemplate: fetch.BranchURLTemplate,

			Sessions: writer,
			// only useful in regular export for progress tracking
			SessionRootID: 0,

			CommitUsers: commitUsers,
		}
		for _, pr1 := range fetch.PRs {
			pr2 := exportrepo.PR{}
			pr2.ID = pr1.ID
			pr2.RefID = pr1.RefID
			pr2.URL = pr1.URL
			pr2.BranchName = pr1.BranchName
			pr2.LastCommitSHA = pr1.LastCommitSHA
			opts.PRs = append(opts.PRs, pr2)
		}
		exp := exportrepo.New(opts, s.opts.Locs)
		runResult := exp.Run(context.Background())
		if runResult.SessionErr != nil {
			res.Err = runResult.SessionErr
			return
		}
		if runResult.OtherErr != nil {
			res.Err = runResult.OtherErr
			return
		}
	}

	res.Data = writer.GetData()
	return
}

type session struct {
	modelType string
	data      []map[string]interface{}
}

type ObjectsWriter struct {
	sessions map[expsessions.ID]session
	lastID   expsessions.ID
	mu       sync.Mutex
}

func NewObjectsWriter() *ObjectsWriter {
	return &ObjectsWriter{
		sessions: map[expsessions.ID]session{},
	}
}

func (s *ObjectsWriter) Session(modelType string, parentSessionID expsessions.ID, parentObjectID, parentObjectName string) (_ expsessions.ID, lastProcessed interface{}, _ error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.newID()
	s.sessions[id] = session{modelType: modelType, data: []map[string]interface{}{}}
	return id, nil, nil
}

func (s *ObjectsWriter) newID() expsessions.ID {
	s.lastID++
	return s.lastID
}

func (s *ObjectsWriter) Write(id expsessions.ID, objs []map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessions[id]
	session.data = append(session.data, objs...)
	s.sessions[id] = session
	return nil
}

func (s *ObjectsWriter) Done(id expsessions.ID, lastProcessed interface{}) error {
	return nil
}

func (s *ObjectsWriter) GetData() rpcdef.MutatedObjects {
	res := rpcdef.MutatedObjects{}
	for _, session := range s.sessions {
		for _, obj := range session.data {
			res[session.modelType] = append(res[session.modelType], obj)
		}
	}
	return res
}
