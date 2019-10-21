package cmdexport

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/cmd/cmdexport/process"
	"github.com/pinpt/agent.next/pkg/commitusers"
	"github.com/pinpt/agent.next/pkg/expsessions"
	"github.com/pinpt/agent.next/pkg/integrationid"
	"github.com/pinpt/agent.next/rpcdef"
)

type sessions struct {
	logger      hclog.Logger
	export      *export
	expsession  *expsessions.Manager
	commitUsers *process.CommitUsers

	progressTracker *expsessions.ProgressTracker
}

func newSessions(logger hclog.Logger, export *export, outputDir string) *sessions {

	s := &sessions{}
	s.logger = logger
	s.export = export
	s.commitUsers = process.NewCommitUsers()

	s.progressTracker = expsessions.NewProgressTracker()

	s.expsession = expsessions.New(expsessions.Opts{
		Logger:        logger,
		LastProcessed: export.lastProcessed,
		NewWriter: func(modelName string, id expsessions.ID) expsessions.Writer {
			return expsessions.NewFileWriter(modelName, outputDir, id)
		},
		SendProgress: func(progressPath expsessions.ProgressPath, current, total int) {
			s.progressTracker.Update(progressPath.StringsWithObjectNames(), current, total)
		},
		SendProgressDone: func(progressPath expsessions.ProgressPath) {
			s.progressTracker.Done(progressPath.StringsWithObjectNames())
		},
	})

	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			<-ticker.C
			res := s.progressTracker.InProgressString()

			if strings.TrimSpace(res) != "" { // do not print empty progress data
				s.logger.Debug("progress", "data", "\n\n"+res+"\n\n")
			}

			if s.export.Opts.AgentConfig.Backend.Enable {
				skipDone := true
				if os.Getenv("PP_AGENT_PROGRESS_ALL") != "" {
					skipDone = false
				}
				res := s.progressTracker.ProgressLinesNestedMap(skipDone)
				b, err := json.Marshal(res)
				if err != nil {
					s.logger.Error("could not marshal progress data", err)
					continue
				}

				//s.logger.Info("progress", "json", "\n\n"+string(b)+"\n\n")
				//continue

				err = s.export.sendProgress(context.Background(), b)
				if err != nil {
					s.logger.Error("could not send progress info to backend", "err", err)
				}
			}
		}
	}()

	return s
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
