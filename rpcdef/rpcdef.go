package rpcdef

import (
	"context"
	"encoding/json"

	"github.com/pinpt/agent2/rpcdef/proto"
	"google.golang.org/grpc"

	"github.com/hashicorp/go-plugin"
)

type Integration interface {
	Init(agent Agent) error
	Export(context.Context) error
}

// handshakeConfigs are used to just do a basic handshake between
// a plugin and host. If the handshake fails, a user friendly error is shown.
// This prevents users from executing bad plugins or executing a plugin
// directory. It is a UX feature, not a security feature.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "PLUGIN",
	MagicCookieValue: "pinpoint-agent-plugin",
}

var PluginMap = map[string]plugin.Plugin{
	"integration": &IntegrationPlugin{},
}

type IntegrationClient struct {
	client          proto.IntegrationClient
	broker          *plugin.GRPCBroker
	agentGRPCServer *grpc.Server
}

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

type Agent interface {
	// SendExported forwards the exported objects from intergration to agent, which then upload the data when necessary
	// modelType is the type of the object. i.e. sourcecode.commit
	// objs is the objects to send in this batch
	SendExported(
		modelType string,
		objs []ExportObj)
}

type ExportObj struct {
	Data interface{}
}

type AgentServer struct {
	Impl Agent
}

func (s *AgentServer) SendExported(ctx context.Context, req *proto.SendExportedReq) (*proto.Empty, error) {
	var objs []ExportObj
	for _, obj := range req.Objs {
		var data interface{}
		err := json.Unmarshal(obj.Data, &data)
		if err != nil {
			return nil, err
		}
		obj2 := ExportObj{}
		obj2.Data = data
		objs = append(objs, obj2)
	}
	s.Impl.SendExported(req.ModelType, objs)
	return &proto.Empty{}, nil
}

type AgentClient struct {
	client proto.AgentClient
}

func (s *AgentClient) SendExported(modelType string, objs []ExportObj) {
	args := &proto.SendExportedReq{}
	args.ModelType = modelType
	for _, obj := range objs {
		obj2 := &proto.ExportObj{}
		obj2.DataType = proto.ExportObj_JSON
		b, err := json.Marshal(obj.Data)
		if err != nil {
			panic(err)
		}
		obj2.Data = b
		args.Objs = append(args.Objs, obj2)
	}

	_, err := s.client.SendExported(context.Background(), args)
	if err != nil {
		panic(err)
	}
}
