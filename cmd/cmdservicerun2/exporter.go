package cmdservicerun2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	pjson "github.com/pinpt/go-common/json"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/cmd/cmdexport"
	"github.com/pinpt/agent.next/pkg/keychain"
	"github.com/pinpt/integration-sdk/agent"
)

type exporterOpts struct {
	Logger       hclog.Logger
	CustomerID   string
	PinpointRoot string
	Encryptor    *keychain.Encryptor
}

type exporter struct {
	ExportQueue chan *agent.ExportRequest

	logger hclog.Logger
	opts   exporterOpts
}

func newExporter(opts exporterOpts) *exporter {
	s := &exporter{}
	s.opts = opts
	s.logger = opts.Logger
	s.ExportQueue = make(chan *agent.ExportRequest)
	return s
}

func (s *exporter) Run() error {
	for data := range s.ExportQueue {
		err := s.export(data)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *exporter) export(data *agent.ExportRequest) error {
	s.logger.Info("processing export request", "upload_url", *data.UploadURL)

	agentConfig := cmdexport.AgentConfig{}
	agentConfig.CustomerID = s.opts.CustomerID
	agentConfig.PinpointRoot = s.opts.PinpointRoot

	var integrations []cmdexport.Integration

	for _, integration := range data.Integrations {
		s.logger.Info("exporting integration", "name", integration.Name)

		in := cmdexport.Integration{}

		if integration.Authorization.Authorization == nil {
			panic("missing encrypted auth data")
		}

		data, err := s.opts.Encryptor.Decrypt(*integration.Authorization.Authorization)
		if err != nil {
			return err
		}

		remoteConfig := map[string]interface{}{}
		err = json.Unmarshal([]byte(data), &remoteConfig)
		if err != nil {
			return err
		}

		s.logger.Debug("integration export", "remote config", remoteConfig)

		name2 := ""
		in.Config, name2 = s.convertConfig(integration.Name, remoteConfig)
		if name2 != "" {
			in.Name = name2
		} else {
			in.Name = integration.Name
		}

		integrations = append(integrations, in)
	}

	ctx := context.Background()
	return s.execExport(ctx, agentConfig, integrations)
}

func (s *exporter) convertConfig(in string, c1 map[string]interface{}) (res map[string]interface{}, integrationName string) {
	res = map[string]interface{}{}
	if in == "github" {
		res["organization"] = "pinpt"
		_, ok := c1["api_token"].(string)
		if !ok {
			panic("missing api_token")
		}
		res["apitoken"] = c1["api_token"]
		_, ok = c1["url"].(string)
		if !ok {
			panic("missing url")
		}
		res["url"] = c1["url"]
		return
	} else if in == "jira" {
		us, ok := c1["url"].(string)
		if !ok {
			panic("missing jira url in config")
		}
		u, err := url.Parse(us)
		if err != nil {
			panic(fmt.Errorf("invlid jira url: %v", err))
		}
		integrationName = "jira-hosted"
		if strings.HasSuffix(u.Host, ".atlassian.net") {
			integrationName = "jira-cloud"
		}
		res["url"] = us
		_, ok = c1["username"].(string)
		if !ok {
			panic("missing username")
		}

		res["username"] = c1["username"]
		_, ok = c1["password"].(string)
		if !ok {
			panic("missing password")
		}

		res["password"] = c1["password"]

		return
	}
	panic("unsupported integration: " + in)
}

func (s *exporter) execExport(ctx context.Context, agentConfig cmdexport.AgentConfig, integrations []cmdexport.Integration) error {
	cmd := exec.CommandContext(ctx, os.Args[0], "export", "--agent-config-json", pjson.Stringify(agentConfig), "--integrations-json", pjson.Stringify(integrations))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
