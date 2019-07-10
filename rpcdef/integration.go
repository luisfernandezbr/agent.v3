package rpcdef

import (
	"context"
	"encoding/json"

	"github.com/pinpt/agent.next/rpcdef/proto"
	"google.golang.org/grpc"

	"github.com/hashicorp/go-plugin"
)

type Integration interface {
	Init(agent Agent) error
	Export(ctx context.Context, exportConfig map[string]interface{}) (ExportResult, error)
}

type ExportResult struct {
	// NewConfig can be returned from Export to update the integration config. Return nil to keep the curren config.
	NewConfig map[string]interface{}
}

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
	exportConfig map[string]interface{}) (res ExportResult, _ error) {

	confBytes, err := json.Marshal(exportConfig)
	if err != nil {
		return res, err
	}

	args := &proto.IntegrationExportReq{}
	args.IntegrationConfigJson = confBytes
	resp, err := s.client.Export(ctx, args)
	if err != nil {
		return res, err
	}
	newConf := resp.IntegrationNewConfigJson
	if len(newConf) != 0 {
		err := json.Unmarshal(newConf, &res.NewConfig)
		if err != nil {
			return res, err
		}
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
	var conf map[string]interface{}
	err := json.Unmarshal(req.IntegrationConfigJson, &conf)
	if err != nil {
		return res, err
	}
	r0, err := s.Impl.Export(ctx, conf)
	if err != nil {
		return res, err
	}
	if r0.NewConfig != nil {
		b, err := json.Marshal(r0.NewConfig)
		if err != nil {
			return res, err
		}
		res.IntegrationNewConfigJson = b
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
