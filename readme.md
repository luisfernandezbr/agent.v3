### Build and run

The instruction below assume you don't have $GOPATH set. If you do replace ~/go with $GOPATH.

Clone
```
mkdir -p ~/go/src/github.com/pinpt
cd ~/go/src/github.com/pinpt
git clone git@github.com:pinpt/agent.next.git
```

Build
```
cd ./agent.next
dep ensure -v
go install github.com/pinpt/agent.next
```

Update
```
git pull
dep ensure -v
go install github.com/pinpt/agent.next
```

Run
```
~/go/bin/agent.next enroll <CODE> --channel=dev
~/go/bin/agent.next service-run
```

It will store the data into ~/.pinpoint/next folder.

If you want to re-enroll the agent, delete ~/.pinpoint/next.


### Overview

Agent is run continuously on the target machine. All actions are driven from a backend.

User installs agent using enroll command, which ensures that service is on the the system startup.

Service waits for commands from a backend to validate integrations, start export or get users, repos, projects for use in admin web interface. All these actions are started as a separate process. To implement this we have the following hidden commands which accept json as params.

- export - Export all data of multiple passed integrations.
- validate-config - Validates the configuration by making a test connection.
- export-onboard-data - Exports users, repos or projects based on param for a specified integration. Saves that data into provided file.

### [Exported data](./_docs/exported_data.md)

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

### Running additional integrations in export

You can add an extra configration line to config after running enroll. They will be run when export is requested in service-run. See example below.

```
{
.... existing fields,
"extra_integrations": [{"name":"mock", "config":{"k":"v"}}]
}
```

### Development tips

#### Checking exported data
When checking exported data is it often needed to look for a specific id or some fields. Using zcat with jq is often sufficient.

One problem is that we generate multiple file per each type, and zcat * does not work on MacOS.

The following workaround works for fish shell:

```
# add this function to ~/.config/fish/config.fish

# similar to zcat * | jq .
# this is required because on MacOS you need to use zcat < file, otherwise zcat wants .Z suffix attached
# Usage:
# in ./sourcecode.Branch
# zcatall | less
function zcatall
	if test -e ./zcatall
		cat ./zcatall
	else
		for f in *.json.gz; zcat < $f | jq . >> ./zcatall; end; cat ./zcatall
	end
end
```

### Logging

This section will bescribe how logging works starting with lower lovel, which are integrations and how it is passed up to export command and then to service-run.

Integrations log the output using provided hclog.Logger, log output is passed up to export, which outputs the logs to stdout and at the same time saves log file per integrations into --pinpoint-root/logs folder. In these files only output from the last run is saved. Panics in integrations are written both into logs file and repeated in stdout output.

When export is run the log output is shown in stdout and the copy is saved into logs/export file. For export-onboard-data and validate-config the file names match the command.

When service-run command is run it outputs all logs to stdout and saves a copy into logs/service-run file. When it runs the subcommands their behavior doesn't change and logs are saved the same way as described above.

There is a special handing for export sub-command in service-run, in addition to usual log handing, the logs are sent to backend api in batches.

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

### Export sessions and progress
We use nested sessions to keep track of the progress and last processed tokens. We have 2 different session types, model sessions which allow sending the objects to the backend and tracking sessions which are used for progress notification and for keeping last processed tokens from overwriting each other.

Here is the preudo-code for using the sessions from integration. We provide a nicer wrapper that calls these methods underneath.

```
exportStart := time.Now()

// NoModel sessions do not support sending objects, and are used for progress tracking and ensuring that last process tokens are nested. This is important when using ids as last process token.

orgSession, _ := SessionRootTracking("organization")

total, orgs := http.GetOrgs()

for i, org := range orgs {
	every x seconds
		SessionProgress(repoSessionID, i, total)

	repoSession, lastProcessed := Session("pull_request". orgSession, org.ID, org.Name)

	total, repos := http.GetRepos(since: lastProcessed)

	for i, repo := range repos {
		every x seconds
			SessionProgress(repoSession, i, total)
		send when you have a batch of repos
			SessionSend(repoSession, batch)

		prSession, lastProcessedID := Session("pull_request", repoSession, repo.ID, repo.Name)

		total, prs := http.GetRepoPullRequest(repo, since: lastProcessedID)

		// example of using id as last processed
		var lastID
		for i, pr := range  {
			id = pr.id
			every x second
				SessionProgress(prSession, i, total)
			send when you have a batch of pull requests
				SessionSend(prSession, batch)		
		}

		SessionDone(prSession, lastID)
	}

	SessionDone(repoSession, exportStart)
}

SessionDone(orgSession, exportStart)
```

### Agent RPC interface

[See in code](https://github.com/pinpt/agent.next/blob/master/rpcdef/agent.go)

```golang
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

