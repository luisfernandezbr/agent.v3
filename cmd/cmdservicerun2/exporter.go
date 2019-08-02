package cmdservicerun2

import (
	"encoding/json"

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
	opts := cmdexport.Opts{}
	opts.Logger = s.logger
	opts.CustomerID = s.opts.CustomerID
	opts.PinpointRoot = s.opts.PinpointRoot
	for _, integration := range data.Integrations {
		s.logger.Info("exporting integration", "name", integration.Name)

		in := cmdexport.Integration{}
		in.Name = integration.Name

		if in.Name == "jira" {
			in.Name = "jira-hosted"
		}

		if integration.Authorization.Authorization == nil {
			panic("missing encrypted auth data")
		}

		data, err := s.opts.Encryptor.Decrypt(*integration.Authorization.Authorization)
		if err != nil {
			return err
		}
		err = json.Unmarshal([]byte(data), &in.Config)
		if err != nil {
			return err
		}
		in.Config = s.convertConfig(in.Name, in.Config)

		s.logger.Debug("integration export config used", "config", in.Config)
		opts.Integrations = append(opts.Integrations, in)
	}

	return cmdexport.Run(opts)
}

func (s *exporter) convertConfig(in string, c1 map[string]interface{}) map[string]interface{} {
	res := map[string]interface{}{}
	if in == "github" {
		res["organization"] = "pinpt"
		res["apitoken"] = c1["api_token"]
		res["url"] = c1["url"]
		return res
	} else if in == "jira" {
		panic("jira config not implemented")
		//res["organization"] = "pinpt"
		//res["apitoken"] = c1["api_token"]
		//res["url"] = c1["url"]
	}
	panic("unsupported integration: " + in)
}
