package cmdexport

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mitchellh/go-homedir"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/pinpt/agent2/rpcdef"
	"github.com/pinpt/go-common/io"
)

type Opts struct {
	Logger  hclog.Logger
	WorkDir string
}

type export struct {
	logger       hclog.Logger
	pluginClient *plugin.Client
	sessions     *sessions
	integration  rpcdef.Integration

	dirs exportDirs
}

type exportDirs struct {
	sessions string
}

func newExport(opts Opts) *export {
	if opts.WorkDir == "" {
		dir, err := homedir.Dir()
		if err != nil {
			panic(err)
		}
		opts.WorkDir = filepath.Join(dir, ".pinpoint", "v2", "work")
	}

	s := &export{}
	s.logger = opts.Logger
	s.dirs = exportDirs{
		sessions: filepath.Join(opts.WorkDir, "sessions"),
	}
	err := s.setupPlugins()
	if err != nil {
		panic(err)
	}
	s.sessions = newSessions(s.dirs.sessions)

	ctx := context.Background()
	err = s.integration.Export(ctx)
	if err != nil {
		panic(err)
	}
	return s
}

func (s export) Destroy() {
	defer s.pluginClient.Kill()
}

func (s *export) setupPlugins() error {
	client := plugin.NewClient(&plugin.ClientConfig{
		Logger:          s.logger,
		HandshakeConfig: rpcdef.Handshake,
		Plugins:         rpcdef.PluginMap,
		Cmd:             devIntegrationCommand("mock"),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC},
	})
	s.pluginClient = client

	rpcClient, err := client.Client()
	if err != nil {
		return err
	}

	raw, err := rpcClient.Dispense("integration")
	if err != nil {
		return err
	}

	s.integration = raw.(rpcdef.Integration)

	delegate := agentDelegate{
		export: s,
	}

	s.integration.Init(delegate)
	return nil
}

type sessions struct {
	m         map[int]session
	streamDir string
	lastID    int
}

func newSessions(streamDir string) *sessions {
	s := &sessions{}
	s.m = map[int]session{}
	s.streamDir = streamDir
	return s
}

func (s *sessions) new(modelType string) (sessionID string, _ error) {
	s.lastID++
	id := s.lastID

	base := strconv.FormatInt(time.Now().Unix(), 10) + "_" + strconv.Itoa(id) + ".json.gz"
	fn := filepath.Join(s.streamDir, modelType, base)
	err := os.MkdirAll(filepath.Dir(fn), 0777)
	if err != nil {
		return "", err
	}
	stream, err := io.NewJSONStream(fn)
	if err != nil {
		return "", err
	}

	s.m[id] = session{
		state:     "started",
		modelType: modelType,
		stream:    stream,
	}
	return strconv.Itoa(id), nil
}

func (s *sessions) get(sessionID string) session {
	id, err := strconv.Atoi(sessionID)
	if err != nil {
		panic(err)
	}
	return s.m[id]
}

func (s *sessions) Close(sessionID string) error {
	sess := s.get(sessionID)
	err := sess.stream.Close()
	if err != nil {
		return err
	}
	idi, err := strconv.Atoi(sessionID)
	if err != nil {
		return err
	}
	delete(s.m, idi)
	return nil
}

func (s *sessions) Write(sessionID string, objs []rpcdef.ExportObj) error {
	sess := s.get(sessionID)
	for _, obj := range objs {
		err := sess.stream.Write(obj)
		if err != nil {
			return err
		}
	}
	return nil
}

type session struct {
	state     string
	modelType string
	stream    *io.JSONStream
}
