package rpcdef

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pinpt/agent.next/rpcdef/proto"
)

// keep in sync with readme.md
type Agent interface {
	// ExportStarted should be called when starting export for each modelType.
	// It returns session id to be used later when sending objects.
	ExportStarted(modelType string) (sessionID string, lastProcessed interface{})

	// ExportDone should be called when export of a certain modelType is complete.
	// TODO: rename to SessionDone
	ExportDone(sessionID string, lastProcessed interface{})

	// SendExported forwards the exported objects from intergration to agent,
	// which then uploads the data (or queues for uploading).
	// TODO: rename to SessionSend
	SendExported(
		sessionID string,
		objs []ExportObj)

	// SessionStart creates a new export session with optional parent.
	// isTracking is a bool to create a tracking session instead of normal session. Tracking sessions do not allow sending data, they are only used for organizing progress events.
	// name - For normal sessions use model name. For tracking sessions any string is allows, it will be shown in the progress log.
	// parentSessionID - parent session. Can be 0 for root sessions.
	// parentObjectID - id of the parent object. To show in progress logs.
	// parentObjectName - name of the parent object
	SessionStart(isTracking bool, name string, parentSessionID int, parentObjectID, parentObjectName string) (sessionID int, lastProcessed interface{}, _ error)

	// SessionProgress updates progress for a session
	SessionProgress(id int, current, total int) error

	// Integration can ask agent to download and process git repo using ripsrc.
	ExportGitRepo(fetch GitRepoFetch) error

	// OAuthNewAccessToken returns a new access token for integrations with UseOAuth: true. It askes agent to retrieve a new token from backend based on refresh token agent has.
	OAuthNewAccessToken() (token string, _ error)

	SendPauseEvent(msg string) error

	SendContinueEvent(msg string) error
}

type ExportObj struct {
	Data interface{}
}

type GitRepoFetch struct {
	RepoID            string
	UniqueName        string
	RefType           string
	URL               string
	CommitURLTemplate string
	BranchURLTemplate string
	PRs               []GitRepoFetchPR
}

func (s GitRepoFetch) Validate() error {
	var missing []string
	if s.RepoID == "" {
		missing = append(missing, "RepoID")
	}
	if s.UniqueName == "" {
		missing = append(missing, "UniqueName")
	}
	if s.URL == "" {
		missing = append(missing, "URL")
	}
	if s.CommitURLTemplate == "" {
		missing = append(missing, "CommitURLTemplate")
	}
	if s.BranchURLTemplate == "" {
		missing = append(missing, "BranchURLTemplate")
	}
	if len(missing) != 0 {
		return fmt.Errorf("missing required param for GitRepoFetch: %s", strings.Join(missing, ", "))
	}
	for _, pr := range s.PRs {
		err := pr.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

type GitRepoFetchPR struct {
	ID            string
	RefID         string
	URL           string
	LastCommitSHA string
}

func (s GitRepoFetchPR) Validate() error {
	var missing []string
	if s.ID == "" {
		missing = append(missing, "ID")
	}
	if s.RefID == "" {
		missing = append(missing, "RefID")
	}
	if s.URL == "" {
		missing = append(missing, "URL")
	}
	if s.LastCommitSHA == "" {
		missing = append(missing, "LastCommitSHA")
	}
	if len(missing) != 0 {
		return fmt.Errorf("missing required param for GitRepoFetch: %s", strings.Join(missing, ", "))
	}
	return nil
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

func (s *AgentServer) ExportGitRepo(ctx context.Context, req *proto.ExportGitRepoReq) (resp *proto.Empty, _ error) {
	resp = &proto.Empty{}
	fetch := GitRepoFetch{}
	fetch.RepoID = req.RepoId
	fetch.UniqueName = req.UniqueName
	fetch.RefType = req.RefType
	fetch.URL = req.Url
	fetch.CommitURLTemplate = req.CommitUrlTemplate
	fetch.BranchURLTemplate = req.BranchUrlTemplate
	for _, pr := range req.Prs {
		pr2 := GitRepoFetchPR{}
		pr2.ID = pr.Id
		pr2.RefID = pr.RefId
		pr2.URL = pr.Url
		pr2.LastCommitSHA = pr.LastCommitSha
		fetch.PRs = append(fetch.PRs, pr2)
	}
	err := s.Impl.ExportGitRepo(fetch)
	if err != nil {
		return resp, err
	}
	return
}

func (s *AgentServer) SessionStart(ctx context.Context, req *proto.SessionStartReq) (resp *proto.SessionStartResp, _ error) {
	resp = &proto.SessionStartResp{}
	sessionID, lastProcessed, err := s.Impl.SessionStart(req.IsTracking, req.Name, int(req.ParentSessionId), req.ParentObjectId, req.ParentObjectName)
	if err != nil {
		return resp, err
	}
	resp.SessionId = int64(sessionID)
	resp.LastProcessed = lastProcessedMarshal(lastProcessed)
	return resp, nil
}

func (s *AgentServer) SessionProgress(ctx context.Context, req *proto.SessionProgressReq) (resp *proto.Empty, _ error) {
	resp = &proto.Empty{}
	err := s.Impl.SessionProgress(int(req.Id), int(req.Current), int(req.Total))
	return resp, err
}

func (s *AgentServer) OAuthNewAccessToken(ctx context.Context, req *proto.Empty) (*proto.OAuthNewAccessTokenResp, error) {

	token, err := s.Impl.OAuthNewAccessToken()

	resp := &proto.OAuthNewAccessTokenResp{}
	resp.Token = token

	return resp, err
}

func (s *AgentServer) SendPauseEvent(ctx context.Context, req *proto.SendPauseEventReq) (resp *proto.Empty, err error) {
	resp = &proto.Empty{}
	err = s.Impl.SendPauseEvent(req.Message)
	return
}

func (s *AgentServer) SendContinueEvent(ctx context.Context, req *proto.SendContinueEventReq) (resp *proto.Empty, err error) {
	resp = &proto.Empty{}
	err = s.Impl.SendContinueEvent(req.Message)
	return
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

func (s *AgentClient) ExportGitRepo(fetch GitRepoFetch) error {
	err := fetch.Validate()
	if err != nil {
		return err
	}
	args := &proto.ExportGitRepoReq{}
	args.RepoId = fetch.RepoID
	args.UniqueName = fetch.UniqueName
	args.RefType = fetch.RefType
	args.Url = fetch.URL
	args.CommitUrlTemplate = fetch.CommitURLTemplate
	args.BranchUrlTemplate = fetch.BranchURLTemplate
	for _, pr := range fetch.PRs {
		pr2 := &proto.ExportGitRepoPR{}
		pr2.Id = pr.ID
		pr2.RefId = pr.RefID
		pr2.Url = pr.URL
		pr2.LastCommitSha = pr.LastCommitSHA
		args.Prs = append(args.Prs, pr2)
	}
	_, err = s.client.ExportGitRepo(context.Background(), args)
	if err != nil {
		return err
	}
	return nil
}

func (s *AgentClient) SessionStart(isTracking bool, name string, parentSessionID int, parentObjectID, parentObjectName string) (sessionID int, lastProcessed interface{}, _ error) {
	args := &proto.SessionStartReq{}
	args.IsTracking = isTracking
	args.Name = name
	args.ParentSessionId = int64(parentSessionID)
	args.ParentObjectId = parentObjectID
	args.ParentObjectName = parentObjectName
	resp, err := s.client.SessionStart(context.Background(), args)
	if err != nil {
		return 0, nil, err
	}
	return int(resp.SessionId), lastProcessedUnmarshal(resp.LastProcessed), nil
}

func (s *AgentClient) SessionProgress(id int, current, total int) error {
	args := &proto.SessionProgressReq{}
	args.Id = int64(id)
	args.Current = int64(current)
	args.Total = int64(total)
	_, err := s.client.SessionProgress(context.Background(), args)
	if err != nil {
		return err
	}
	return nil
}

func (s *AgentClient) OAuthNewAccessToken() (token string, _ error) {
	args := &proto.Empty{}
	resp, err := s.client.OAuthNewAccessToken(context.Background(), args)
	if err != nil {
		return "", err
	}
	return resp.Token, nil
}

func (s *AgentClient) SendPauseEvent(msg string) error {
	args := &proto.SendPauseEventReq{
		Message: msg,
	}
	_, err := s.client.SendPauseEvent(context.Background(), args)
	if err != nil {
		return err
	}
	return nil
}

func (s *AgentClient) SendContinueEvent(msg string) error {
	args := &proto.SendContinueEventReq{
		Message: msg,
	}
	_, err := s.client.SendContinueEvent(context.Background(), args)
	if err != nil {
		return err
	}
	return nil
}
