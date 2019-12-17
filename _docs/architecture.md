### Overview
Agent is run continuously on the target machine. All actions are driven from a backend.

User installs agent using enroll command, which ensures that service is on the the system startup.

Service waits for commands from a backend to validate integrations, start export or get users, repos, projects for use in admin web interface. All these actions are started as a separate process. To implement this we have the following hidden commands which accept json as params.

- export - Export all data of multiple passed integrations.
- validate-config - Validates the configuration by making a test connection.
- export-onboard-data - Exports users, repos or projects based on param for a specified integration. Saves that data into provided file.

### Logging
This section describes how logging works starting with lower lovel, which are integrations and how it is passed up to export command and then to service-run.

Integrations log the output using provided hclog.Logger, log output is passed up to export, which outputs the logs to stdout and at the same time saves log file per integrations into --pinpoint-root/logs folder. In these files only output from the last run is saved. Panics in integrations are written both into logs file and repeated in stdout output.

When export is run the log output is shown in stdout and the copy is saved into logs/export file. For export-onboard-data and validate-config the file names match the command.

When service-run command is run it outputs all logs to stdout and saves a copy into logs/service-run file. When it runs the subcommands their behavior doesn't change and logs are saved the same way as described above.

There is a special handing for export sub-command in service-run, in addition to usual log handing, the logs are sent to backend api in batches.

### Data format
GRPC is used for calls between agent and integrations. Endpoints and parameters are defined using .proto files.

Integrations are responsible for getting the data and converting it to pinpoint format. Integrations will use datamodel directly. Agent itself does not need to check the datamodel. Agent will use the metadata to correctly forward that data to backend, but does not have to touch the data itself.

### RPC interface between agent and integration
- [Agent](https://github.com/pinpt/agent/blob/master/rpcdef/agent.go)
- [Integration](https://github.com/pinpt/agent/blob/master/rpcdef/integration.go)

### Export code flow
When agent export command is called, agent loads all available/configured plugins and then inits them using the Init call to allow them to call back to the agent.

After that agent calls Export methods on integrations in parallel. Integration marks the export state for each model type using ExportStarted and ExportDone. It sends the data using SendExported call.

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