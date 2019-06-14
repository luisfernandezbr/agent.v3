package rpcdef

import (
	"context"

	"github.com/pinpt/agent2/rpcdef/proto"
	"google.golang.org/grpc"

	"github.com/hashicorp/go-plugin"
)

type Integration interface {
	Init(agent Agent) error
	Export(context.Context) error
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

func (s *IntegrationClient) Export(ctx context.Context) error {
	_, err := s.client.Export(ctx, &proto.Empty{})
	return err
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

func (s *IntegrationServer) Export(ctx context.Context, req *proto.Empty) (*proto.Empty, error) {
	err := s.Impl.Export(nil)
	return &proto.Empty{}, err
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
