package cmdservicerun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/pinpt/agent.next/pkg/encrypt"
	"github.com/pinpt/agent.next/pkg/structmarshal"

	"github.com/pinpt/agent.next/cmd/cmdupload"

	"github.com/pinpt/agent.next/pkg/fsconf"

	pjson "github.com/pinpt/go-common/json"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/cmd/cmdexport"
	"github.com/pinpt/integration-sdk/agent"
)

type exporterOpts struct {
	Logger       hclog.Logger
	CustomerID   string
	PinpointRoot string
	FSConf       fsconf.Locs

	PPEncryptionKey string
}

type exporter struct {
	ExportQueue chan exportRequest

	logger hclog.Logger
	opts   exporterOpts
}

type exportRequest struct {
	Done chan error
	Data *agent.ExportRequest
}

func newExporter(opts exporterOpts) *exporter {
	if opts.PPEncryptionKey == "" {
		panic(`opts.PPEncryptionKey == ""`)
	}
	s := &exporter{}
	s.opts = opts
	s.logger = opts.Logger
	s.ExportQueue = make(chan exportRequest)
	return s
}

func (s *exporter) Run() {
	for req := range s.ExportQueue {
		req.Done <- s.export(req.Data)
	}
	return
}

func (s *exporter) export(data *agent.ExportRequest) error {
	s.logger.Info("processing export request", "upload_url", *data.UploadURL)

	agentConfig := cmdexport.AgentConfig{}
	agentConfig.CustomerID = s.opts.CustomerID
	agentConfig.PinpointRoot = s.opts.PinpointRoot

	var integrations []cmdexport.Integration

	/*
		integrations = append(integrations, cmdexport.Integration{
			Name:   "mock",
			Config: map[string]interface{}{"k1": "v1"},
		})
	*/

	for _, integration := range data.Integrations {
		// TODO: using mock above instead
		//continue

		s.logger.Info("exporting integration", "name", integration.Name)

		in := cmdexport.Integration{}

		if integration.Authorization.Authorization == nil {
			panic("missing encrypted auth data")
		}

		data, err := encrypt.DecryptString(*integration.Authorization.Authorization, s.opts.PPEncryptionKey)
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
		in.Config, name2 = s.convertConfig(integration.Name, remoteConfig, integration.Exclusions)
		if name2 != "" {
			in.Name = name2
		} else {
			in.Name = integration.Name
		}

		integrations = append(integrations, in)
	}

	ctx := context.Background()

	fsconf := s.opts.FSConf

	// delete existing uploads
	err := os.RemoveAll(fsconf.Uploads)
	if err != nil {
		return err
	}

	err = s.execExport(ctx, agentConfig, integrations)
	if err != nil {
		return err
	}

	s.logger.Info("export finished, running upload")

	err = cmdupload.Run(ctx, s.logger, s.opts.PinpointRoot, *data.UploadURL)
	if err != nil {
		return err
	}
	return nil
}

func (s *exporter) convertConfig(in string, c1 map[string]interface{}, exclusions []string) (res map[string]interface{}, integrationName string) {
	if in == "github" {
		var config struct {
			URL           string   `json:"url"`
			APIToken      string   `json:"apitoken"`
			Organization  string   `json:"organization"`
			ExcludedRepos []string `json:"excluded_repos"`
		}
		err := structmarshal.MapToStruct(c1, &config)
		if err != nil {
			panic(err)
		}
		config.Organization = "pinpt"
		{
			v, ok := c1["api_token"].(string)
			if !ok {
				panic("missing api_token")
			}
			config.APIToken = v
		}
		{
			v, ok := c1["url"].(string)
			if !ok {
				panic("missing url")
			}
			config.URL = v
		}
		config.ExcludedRepos = exclusions
		res, err := structmarshal.StructToMap(config)
		if err != nil {
			panic(err)
		}
		return res, ""
	} else if in == "jira" {
		var config struct {
			URL              string   `json:"url"`
			Username         string   `json:"username"`
			Password         string   `json:"password"`
			ExcludedProjects []string `json:"excluded_projects"`
		}
		err := structmarshal.MapToStruct(c1, &config)
		if err != nil {
			panic(err)
		}
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
		config.URL = us
		{
			v, ok := c1["username"].(string)
			if !ok {
				panic("missing username")
			}
			config.Username = v
		}
		{
			v, ok := c1["password"].(string)
			if !ok {
				panic("missing password")
			}
			config.Password = v
		}
		config.ExcludedProjects = exclusions
		res, err := structmarshal.StructToMap(config)
		if err != nil {
			panic(err)
		}
		return res, integrationName
	}
	panic("unsupported integration: " + in)
}

func (s *exporter) execExport(ctx context.Context, agentConfig cmdexport.AgentConfig, integrations []cmdexport.Integration) error {
	cmd := exec.CommandContext(ctx, os.Args[0], "export", "--agent-config-json", pjson.Stringify(agentConfig), "--integrations-json", pjson.Stringify(integrations))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
