package rpcdef

import (
	"context"
	"encoding/json"

	"github.com/pinpt/agent.next/rpcdef/proto"
)

// keep in sync with readme.md
type Agent interface {
	// ExportStarted should be called when starting export for each modelType.
	// It returns session id to be used later when sending objects.
	ExportStarted(modelType string) (sessionID string, lastProcessed interface{})

	// ExportDone should be called when export of a certain modelType is complete.
	ExportDone(sessionID string, lastProcessed interface{})

	// SendExported forwards the exported objects from intergration to agent,
	// which then uploads the data (or queues for uploading).
	SendExported(
		sessionID string,
		objs []ExportObj)

	// Integration can ask agent to download and process git repo using ripsrc.
	ExportGitRepo(fetch GitRepoFetch)
}

type ExportObj struct {
	Data interface{}
}

type GitRepoFetch struct {
	URL string
}

type AgentServer struct {
	Impl Agent
}

func lastProcessedMarshal(data interface{}) *proto.LastProcessed {
	if data == nil {
		return &proto.LastProcessed{}
	}
	switch data.(type) {
	case string:
		return &proto.LastProcessed{DataStr: data.(string)}
	default:
		panic("data type not supported")
	}
}

func lastProcessedUnmarshal(obj *proto.LastProcessed) interface{} {
	if obj.DataStr != "" {
		return obj.DataStr
	}
	return nil
}

func (s *AgentServer) ExportStarted(ctx context.Context, req *proto.ExportStartedReq) (*proto.ExportStartedResp, error) {

	sessionID, lastProcessed := s.Impl.ExportStarted(req.ModelType)

	resp := &proto.ExportStartedResp{}
	resp.SessionId = sessionID
	resp.LastProcessed = lastProcessedMarshal(lastProcessed)

	return resp, nil
}

func (s *AgentServer) ExportDone(ctx context.Context, req *proto.ExportDoneReq) (*proto.Empty, error) {

	s.Impl.ExportDone(req.SessionId, lastProcessedUnmarshal(req.LastProcessed))

	resp := &proto.Empty{}
	return resp, nil
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

	s.Impl.SendExported(
		req.SessionId,
		objs)

	return &proto.Empty{}, nil
}

func (s *AgentServer) ExportGitRepo(ctx context.Context, req *proto.ExportGitRepoReq) (*proto.Empty, error) {
	fetch := GitRepoFetch{}
	fetch.URL = req.Url
	s.Impl.ExportGitRepo(fetch)
	resp := &proto.Empty{}
	return resp, nil
}

type AgentClient struct {
	client proto.AgentClient
}

var _ Agent = (*AgentClient)(nil)

func (s *AgentClient) ExportStarted(modelType string) (sessionID string, lastProcessed interface{}) {
	args := &proto.ExportStartedReq{}
	args.ModelType = modelType
	resp, err := s.client.ExportStarted(context.Background(), args)
	if err != nil {
		panic(err)
	}
	return resp.SessionId, lastProcessedUnmarshal(resp.LastProcessed)
}

func (s *AgentClient) ExportDone(sessionID string, lastProcessed interface{}) {
	args := &proto.ExportDoneReq{}
	args.SessionId = sessionID
	args.LastProcessed = lastProcessedMarshal(lastProcessed)
	_, err := s.client.ExportDone(context.Background(), args)
	if err != nil {
		panic(err)
	}
}

func (s *AgentClient) SendExported(sessionID string, objs []ExportObj) {
	args := &proto.SendExportedReq{}
	args.SessionId = sessionID
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

func (s *AgentClient) ExportGitRepo(fetch GitRepoFetch) {
	args := &proto.ExportGitRepoReq{}
	args.Url = fetch.URL
	_, err := s.client.ExportGitRepo(context.Background(), args)
	if err != nil {
		panic(err)
	}
}
