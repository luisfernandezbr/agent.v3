### Overview

Agent is run continuosly on the target machine. All actions are driven from a backend.

User installs agent using enroll command, which ensures that service is on the the system startup.

Service waits for commands from a backend to validate integrations, start export or get users, repos, projects for use in admin web interface. All these actions are started as a separate process. To implement this we have the following hidden commands which accept json as params.

- export - Export all data of multiple passed integrations.
- validate-config - Validates the configuration by making a test connection.
- export-onboard-data - Exports users, repos or projects based on param for a specified integration. Saves that data into provided file.

### Testing integrations without backend

```
Windows powershell
Export
.\agent-next.exe export 2>&1 > logs.txt --% --agent-config-json="{\"customer_id\":\"c1\"}" --integrations-json="[{\"name\":\"mock\", \"config\":{\"k\":\"v\"}}]" --pinpoint-root=.

Onboarding data
.\agent-next.exe export-onboard-data 2>&1 > logs.txt --% --agent-config-json="{\"customer_id\":\"c1\",\"skip_git\":true}" --integrations-json="[{\"name\":\"jira-hosted\", \"config\":{\"username\":\"XXX\", \"password\":\"XXX\", \"url\":\"https://xxxxxxxxxxxxxx\"}}]" --pinpoint-root=. --object-type=projects

Getting logs
Get-Content .\logs.txt -Wait -Tail 10
```

### Using separate processes for executing commands in service
We have a long running service that accepts commands from the backend, such as export, validation, getting users and similar. We could run these directly or as a separate processes.

Advantages of using processes
- Errors in export wrapping code would not crash service. Integrations are separate, but ripsrc currently runs in process. If we move ripsrc into integration then this would not be a large advantage because the export parent code will be relatively small.
- Integrations plugins are cleaned up every time. No resource or memory leaks this way.
- Loading integration plugins every time takes acceptable time for every command we do.

Advantages of direct calls
- Can pass callbacks, for example handle progress updates in the service.
- Easy passing/return of data and logs. Simple streaming data back from export-onboard-data.

At the moment, we do not need communications between processes, or complex communication with service. For this reason using processes is a bit better.

### Data format

GRPC is used for calls between agent and integrations. Endpoints and parameters are defined using .proto files.

Integrations are responsible for getting the data and converting it to pinpoint format. Integrations will use datamodel directly. Agent itself does not need to check the datamodel. Agent will use the metadata to correctly forward that data to backend, but does not have to touch the data itself.

### Export code flow

When agent export command is called, agent loads all available/configured plugins and then inits them using the Init call to allow them to call back to the agent.

After that agent calls Export methods on integrations in parallel. Integration marks the export state for each model type using ExportStarted and ExportDone. It sends the data using SendExported call.

### Agent RPC interface

[See in code](https://github.com/pinpt/agent.next/blob/master/rpcdef/agent.go)

```golang
type Agent interface {

// ExportStarted should be called when starting export for each modelType.
// It returns session id to be used later when sending objects.
ExportStarted(modelType string) 
	(sessionID string,
	lastProcessed interface{})

// ExportDone should be called when export of a certain modelType is complete.
ExportDone(sessionID string, lastProcessed interface{})

// SendExported forwards the exported objects from intergration to agent,
// which then uploads the data (or queues for uploading).
SendExported(
		sessionID string,
		objs []ExportObj)

// Integration can ask agent to download and process git repo using ripsrc.
ExportGitRepo(fetch GitRepoFetch) error

}

```


### Integration RPC interface

[See in code](https://github.com/pinpt/agent.next/blob/master/rpcdef/integration.go)

```golang
type Integration interface {

// Init provides the connection details for connecting back to agent.
Init(connectionDetails)

// Export starts export of all data types for this integration.
// Config contains typed config common for all integrations and map[string]interface{} for custom fields.
Export(context.Context, ExportConfig) (ExportResult, error)

type ExportConfig struct {
	Pinpoint    ExportConfigPinpoint
	Integration map[string]interface{}
}

type ExportConfigPinpoint struct {
	CustomerID string
}

//type ExportResult struct {
	// NewConfig can be returned from Export to update the integration config. Return nil to keep the current config.
	//NewConfig map[string]interface{}
//}

ValidateConfig(context.Context, ExportConfig) (ValidationResult, error)

type ValidationResult struct {
	Errors []string
}

// OnboardExport returns the data used in onboard. Kind is one of users, repos, projects. 
OnboardExport(ctx context.Context, kind string, config ExportConfig) (OnboardExportResult, error)

// OnboardExportResult is the result of the onboard call. If the particular data type is not supported by integration, return Error will be equal to OnboardExportErrNotSupported.
type OnboardExportResult struct {
	Error error
	Records []byte 
}

var OnboardExportErrNotSupported

```

