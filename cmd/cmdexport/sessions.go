package cmdexport

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/cmd/cmdexport/process"
	"github.com/pinpt/agent/pkg/commitusers"
	"github.com/pinpt/agent/pkg/expsessions"
	"github.com/pinpt/agent/pkg/integrationid"
	"github.com/pinpt/agent/rpcdef"
)

type sessions struct {
	logger      hclog.Logger
	export      *export
	expsession  *expsessions.Manager
	commitUsers *process.CommitUsers

	progressTracker *expsessions.ProgressTracker

	dedupStore expsessions.DedupStore

	trackProgress bool
}

func newSessions(logger hclog.Logger, export *export, reprocessHistorical bool, trackProgress bool) (_ *sessions, rerr error) {

	s := &sessions{}
	s.logger = logger
	s.export = export
	s.commitUsers = process.NewCommitUsers()
	s.trackProgress = trackProgress

	if s.trackProgress {
		s.progressTracker = expsessions.NewProgressTracker()
	}

	newWriter := func(modelName string, id expsessions.ID) expsessions.Writer {
		return expsessions.NewFileWriter(modelName, export.Locs.Uploads, id)
	}

	// we dedup objects in incremental processing, as perf optimization do not store hashes for hitorical export
	if !reprocessHistorical && os.Getenv("PP_AGENT_DISABLE_DEDUP") == "" {
		var err error
		s.dedupStore, err = expsessions.NewDedupStore(export.Locs.DedupFile)
		if err != nil {
			rerr = err
			return
		}
		newWriterPrev := newWriter
		newWriter = func(modelName string, id expsessions.ID) expsessions.Writer {
			wr := newWriterPrev(modelName, id)
			return expsessions.NewWriterDedup(wr, s.dedupStore)
		}
	}

	s.expsession = expsessions.New(expsessions.Opts{
		Logger:        logger,
		LastProcessed: export.lastProcessed,
		NewWriter:     newWriter,
		SendProgress: func(progressPath expsessions.ProgressPath, current, total int) {
			if s.trackProgress {
				s.progressTracker.Update(progressPath.StringsWithObjectNames(), current, total)
			}
		},
		SendProgressDone: func(progressPath expsessions.ProgressPath) {
			if s.trackProgress {
				s.progressTracker.Done(progressPath.StringsWithObjectNames())
			}
		},
	})

	if s.trackProgress {

		ticker := time.NewTicker(10 * time.Second)
		go func() {
			for {
				<-ticker.C
				s.sendProgress()
			}
		}()

	}

	return s, nil
}

func (s *sessions) sendProgress() {
	res := s.progressTracker.InProgressString()

	if strings.TrimSpace(res) != "" { // do not print empty progress data
		s.logger.Debug("progress", "data", "\n\n"+res+"\n\n")
	}

	if s.export.Opts.AgentConfig.Backend.Enable {
		skipDone := false
		if os.Getenv("PP_AGENT_NO_PROGRESS_ALL") != "" {
			skipDone = true
		}
		//if os.Getenv("PP_AGENT_PROGRESS_ALL") != "" {
		//	skipDone = false
		//}
		res := s.progressTracker.ProgressLinesNestedMap(skipDone)
		b, err := json.Marshal(res)
		if err != nil {
			s.logger.Error("could not marshal progress data", err)
			return
		}

		//s.logger.Info("progress", "json", "\n\n"+string(b)+"\n\n")
		//continue

		err = s.export.sendProgress(context.Background(), b)
		if err != nil {
			s.logger.Error("could not send progress info to backend", "err", err)
		}
	}
}

func (s *sessions) Close() error {

	if s.trackProgress {
		// send last process data at the end when complete
		s.sendProgress()
	}

	if s.dedupStore != nil {
		newObjs, dups := s.dedupStore.Stats()
		s.logger.Info("Dedup stats", "duplicates", dups, "new", newObjs)

		err := s.dedupStore.Save()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *sessions) new(in integrationid.ID, modelType string) (
	sessionID string, lastProcessed interface{}, _ error) {

	id, lastProcessed, err := s.expsession.SessionRoot(in, modelType)
	if err != nil {
		return "", nil, err
	}
	return idToString(id), lastProcessed, nil
}

func (s *sessions) ExportDone(sessionID string, lastProcessed interface{}) error {
	id := idFromString(sessionID)
	return s.expsession.Done(id, lastProcessed)
}

func idToString(id expsessions.ID) string {
	return strconv.Itoa(int(id))
}

func idFromString(str string) expsessions.ID {
	id, err := strconv.Atoi(str)
	if err != nil {
		panic(err)
	}
	return expsessions.ID(id)
}

func (s *sessions) Write(sessionID string, objs []rpcdef.ExportObj) error {
	if len(objs) == 0 {
		//s.logger.Debug("no objects passed to write")
		return nil
	}

	id := idFromString(sessionID)
	modelType := s.expsession.GetModelType(id)
	//s.logger.Info("writing objects", "type", modelType, "count", len(objs))

	if modelType == commitusers.TableName {
		var res []rpcdef.ExportObj
		for _, obj := range objs {
			obj2, err := s.commitUsers.Transform(obj.Data.(map[string]interface{}))
			if err != nil {
				return err
			}
			if obj2 != nil {
				res = append(res, rpcdef.ExportObj{Data: obj2})
			}
		}
		if len(res) == 0 {
			// no new users
			return nil
		}
		objs = res
	}

	var data []map[string]interface{}
	for _, obj := range objs {
		data = append(data, obj.Data.(map[string]interface{}))
	}
	return s.expsession.Write(id, data)
}
