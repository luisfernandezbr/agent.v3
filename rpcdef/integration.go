package rpcdef

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pinpt/agent.next/rpcdef/proto"
	"google.golang.org/grpc"

	"github.com/hashicorp/go-plugin"
)

type Integration interface {
	Init(agent Agent) error
	Export(context.Context, ExportConfig) (ExportResult, error)
	ValidateConfig(context.Context, ExportConfig) (ValidationResult, error)
	// OnboardExport returns the data used in onboard. Kind is one of users, repos, projects.
	OnboardExport(ctx context.Context, objectType OnboardExportType, config ExportConfig) (OnboardExportResult, error)
}

type ExportConfig struct {
	Pinpoint    ExportConfigPinpoint
	Integration map[string]interface{}
}

func exportConfigFromProto(data *proto.IntegrationExportConfig) (res ExportConfig, _ error) {
	err := json.Unmarshal(data.IntegrationConfigJson, &res.Integration)
	if err != nil {
		return res, err
	}
	res.Pinpoint.CustomerID = data.AgentConfig.CustomerId
	return res, nil
}

func (s ExportConfig) proto() (*proto.IntegrationExportConfig, error) {
	res := &proto.IntegrationExportConfig{}
	b, err := json.Marshal(s.Integration)
	if err != nil {
		return res, err
	}
	res.IntegrationConfigJson = b
	res.AgentConfig = s.Pinpoint.proto()
	return res, nil
}

type ExportConfigPinpoint struct {
	CustomerID string
}

func (s ExportConfigPinpoint) proto() *proto.IntegrationAgentConfig {
	res := &proto.IntegrationAgentConfig{}
	res.CustomerId = s.CustomerID
	return res
}

type ExportResult struct {
}

type ValidationResult struct {
	Errors []string `json:"errors"`
}

type OnboardExportType string

const (
	OnboardExportTypeUsers    OnboardExportType = "users"
	OnboardExportTypeRepos                      = "repos"
	OnboardExportTypeProjects                   = "projects"
)

func onboardExportTypeFromProto(k proto.IntegrationOnboardExportReq_Kind) (res OnboardExportType) {
	switch k {
	case proto.IntegrationOnboardExportReq_USERS:
		return OnboardExportTypeUsers
	case proto.IntegrationOnboardExportReq_REPOS:
		return OnboardExportTypeRepos
	case proto.IntegrationOnboardExportReq_PROJECTS:
		return OnboardExportTypeProjects
	default:
		panic(fmt.Errorf("unsupported object type: %v", k))
	}
}

func (s OnboardExportType) proto() proto.IntegrationOnboardExportReq_Kind {
	switch s {
	case OnboardExportTypeUsers:
		return proto.IntegrationOnboardExportReq_USERS
	case OnboardExportTypeRepos:
		return proto.IntegrationOnboardExportReq_REPOS
	case OnboardExportTypeProjects:
		return proto.IntegrationOnboardExportReq_PROJECTS
	default:
		panic(fmt.Errorf("unsupported object type: %v", s))
	}
}

// OnboardExportResult is the result of the onboard call. If the particular data type is not supported by integration, return Error will be equal to OnboardExportErrNotSupported.
type OnboardExportResult struct {
	Error   error
	Records []map[string]interface{}
}

var ErrOnboardExportNotSupported = errors.New("onboard for integration does not support requested object type")

type IntegrationClient struct {
	client          proto.IntegrationClient
	broker          *plugin.GRPCBroker
	agentGRPCServer *grpc.Server
}

var _ Integration = (*IntegrationClient)(nil)

func NewIntegrationClient(protoClient proto.IntegrationClient, broker *plugin.GRPCBroker) *IntegrationClient {
	s := &IntegrationClient{}
	s.client = protoClient
	s.broker = broker
	return s
}

func (s *IntegrationClient) Init(agent Agent) error {
	server := &AgentServer{Impl: agent}
	serverFunc := func(opts []grpc.ServerOption) *grpc.Server {
		gs := grpc.NewServer(opts...)
		proto.RegisterAgentServer(gs, server)
		s.agentGRPCServer = gs
		return gs
	}
	brokerID := s.broker.NextId()
	go s.broker.AcceptAndServe(brokerID, serverFunc)

	args := &proto.IntegrationInitReq{ServerId: brokerID}
	_, err := s.client.Init(context.Background(), args)
	return err
}

func (s *IntegrationClient) Destroy() {
	s.agentGRPCServer.Stop()
}

func (s *IntegrationClient) Export(
	ctx context.Context,
	exportConfig ExportConfig) (res ExportResult, _ error) {

	args := &proto.IntegrationExportReq{}
	var err error
	args.Config, err = exportConfig.proto()
	if err != nil {
		return res, err
	}
	_, err = s.client.Export(ctx, args)
	if err != nil {
		return res, err
	}
	return res, nil
}

func (s *IntegrationClient) ValidateConfig(ctx context.Context,
	exportConfig ExportConfig) (res ValidationResult, _ error) {
	args := &proto.IntegrationValidateConfigReq{}
	var err error
	args.Config, err = exportConfig.proto()
	resp, err := s.client.ValidateConfig(ctx, args)
	if err != nil {
		return res, err
	}
	res.Errors = resp.Errors
	return res, nil
}

func (s *IntegrationClient) OnboardExport(ctx context.Context, objectType OnboardExportType, exportConfig ExportConfig) (res OnboardExportResult, _ error) {
	args := &proto.IntegrationOnboardExportReq{}
	var err error
	args.Config, err = exportConfig.proto()
	args.Kind = objectType.proto()
	resp, err := s.client.OnboardExport(ctx, args)
	if err != nil {
		return res, err
	}
	switch resp.Error {
	case proto.IntegrationOnboardExportResp_NONE:
		res.Error = nil
	case proto.IntegrationOnboardExportResp_NOT_SUPPORTED:
		res.Error = ErrOnboardExportNotSupported
	}

	err = json.Unmarshal(resp.RecordsJson, &res.Records)
	if err != nil {
		return res, err
	}
	return res, nil
}

type IntegrationServer struct {
	Impl   Integration
	broker *plugin.GRPCBroker

	conn *grpc.ClientConn
}

func NewIntegrationServer(impl Integration, broker *plugin.GRPCBroker) *IntegrationServer {
	return &IntegrationServer{
		Impl:   impl,
		broker: broker,
	}
}

func (s *IntegrationServer) Destroy() error {
	return s.conn.Close()
}

func (s *IntegrationServer) Init(ctx context.Context, req *proto.IntegrationInitReq) (*proto.Empty, error) {
	conn, err := s.broker.Dial(req.ServerId)
	if err != nil {
		return nil, err
	}
	as := &AgentClient{proto.NewAgentClient(conn)}
	err = s.Impl.Init(as)
	return &proto.Empty{}, err
}

func (s *IntegrationServer) Export(ctx context.Context, req *proto.IntegrationExportReq) (res *proto.IntegrationExportResp, _ error) {
	res = &proto.IntegrationExportResp{}

	config, err := exportConfigFromProto(req.Config)
	if err != nil {
		return res, err
	}
	_, err = s.Impl.Export(ctx, config)
	if err != nil {
		return res, err
	}
	return res, nil
}

func (s *IntegrationServer) ValidateConfig(ctx context.Context, req *proto.IntegrationValidateConfigReq) (res *proto.IntegrationValidateConfigResp, _ error) {
	res = &proto.IntegrationValidateConfigResp{}

	config, err := exportConfigFromProto(req.Config)
	if err != nil {
		return res, err
	}
	r0, err := s.Impl.ValidateConfig(ctx, config)
	if err != nil {
		return res, err
	}
	res.Errors = r0.Errors
	return res, nil
}

func (s *IntegrationServer) OnboardExport(ctx context.Context, req *proto.IntegrationOnboardExportReq) (res *proto.IntegrationOnboardExportResp, _ error) {
	res = &proto.IntegrationOnboardExportResp{}

	config, err := exportConfigFromProto(req.Config)
	if err != nil {
		return res, err
	}
	kind := onboardExportTypeFromProto(req.Kind)
	r0, err := s.Impl.OnboardExport(ctx, kind, config)
	if err != nil {
		return res, err
	}
	switch r0.Error {
	case nil:
		res.Error = proto.IntegrationOnboardExportResp_NONE
	case ErrOnboardExportNotSupported:
		res.Error = proto.IntegrationOnboardExportResp_NOT_SUPPORTED
	default:
		return res, r0.Error
	}
	res.RecordsJson, err = json.Marshal(r0.Records)
	if err != nil {
		return res, err
	}
	return res, nil
}

type IntegrationPlugin struct {
	plugin.Plugin
	Impl Integration
}

func (s *IntegrationPlugin) GRPCServer(broker *plugin.GRPCBroker, server *grpc.Server) error {
	is := NewIntegrationServer(s.Impl, broker)
	proto.RegisterIntegrationServer(server, is)
	return nil
}

func (s *IntegrationPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	cl := proto.NewIntegrationClient(c)
	return NewIntegrationClient(cl, broker), nil
}
