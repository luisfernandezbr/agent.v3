### Data format

GRPC is used for calls between agent and integrations. Endpoints and parameters are defined using .proto files.

Integrations are responsible for getting the data and converting it to pinpoint format. Integrations will use datamodel directly. Agent itself does not need to check the datamodel. Agent will use the metadata to correctly forward that data to backend, but does not have to touch the data itself.

### Export code flow

When agent export command is called, agent loads all available/configured plugins and then inits them using the Init call to allow them to call back to the agent.

After that agent calls Export methods on integrations in parallel. Integration marks the export state for each model type using ExportStarted and ExportDone. It sends the data using SendExported call.

### Agent RPC interface

```golang
type Agent interface {

// ExportStarted should be called when starting export for each modelType.
// It returns session id to be used later when sending objects.
ExportStarted(modelType string) (sessionID string)

// ExportDone should be called when export of a certain modelType is complete.
ExportDone(sessionID string)

// SendExported forwards the exported objects from intergration to agent,
// which then uploads the data (or queues for uploading).
SendExported(sessionID string, objs interface{}, lastProcessedToken string)

// Integration can ask agent to download and process git repo using ripsrc.
ExportGitRepo(creds Creds)

}
```

### Integration RPC interface

```golang
type Integration interface {

// Init provides the connection details for connecting back to agent.
Init(connectionDetails)

// Export starts the export of all data types for this integration.
Export()
```