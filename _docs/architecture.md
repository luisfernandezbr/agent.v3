### Overview
Agent is run continuously on the customer or pinpoint server. All actions are driven from a backend.

User installs agent using enroll command, which creates the config file that will be reused on restarts.

Service waits for commands from a backend to validate integrations, start export or get users, repos, projects for use in web app. All these actions are started as a separate process. To implement this we have the following hidden commands which accept json as params.

- export - Export all data of multiple passed integrations.
- validate-config - Validates the configuration by making a test connection.
- export-onboard-data - Exports users, repos or projects based on param for a specified integration. Saves that data into provided file.
- mutate - Writes data back to source system
- webhook - Event driven data export

### Data format
GRPC is used for calls between agent and integrations. Endpoints and parameters are defined using .proto files.

Integrations are responsible for getting the data and converting it to pinpoint format. Integrations use the datamodel directly. Agent itself does not need to check the datamodel. Agent uses the metadata to correctly forward that data to backend, but does not modify the data itself, with exception of users, that get special treatment for users created from git commits.

### RPC interface between agent and integration
- [Agent](https://github.com/pinpt/agent/blob/master/rpcdef/agent.go)
- [Integration](https://github.com/pinpt/agent/blob/master/rpcdef/integration.go)

### Export code flow
When agent export command is called, agent loads all available/configured integrations and then inits them using the Init call to allow them to call back to the agent.

After that agent calls Export methods on integrations in parallel. Integration marks the export state for each model type using ExportStarted, ExportDone and Session* methods. It sends the data using SendExported call.

### Logging
This section describes how logging works starting with lower level in integration binaries and how it is passed up to export command and then to command run.

Integrations log the output using provided hclog.Logger, log output is passed up to export, which outputs the logs to stdout and at the same time saves log file per integrations into --pinpoint-root/logs folder. In these files only output from the last run is saved. Panics in integrations are written into log files and repeated in stdout output.

When export is run the log output is shown in stdout and the copy is saved into logs/export file. For export-onboard-data and validate-config the file names match the command.

When run command starts it outputs all logs to stdout and saves a copy into logs/run file. When it runs the subcommands their behavior doesn't change and logs are saved the same way as described above.

There is a special handing for export sub-command in run command, in addition to usual log handing, the logs are sent to the backend api in batches. You can access them using Kibana or download from Elasticsearch.

### Using separate processes for executing commands in service
We have a long running service that accepts commands from the backend, such as export, validation, getting users and similar. We could run these directly or as a separate processes.

Advantages of using processes
- Errors in export wrapping code would not crash service. Integrations are separate, but ripsrc currently runs in process. If we move ripsrc into integration then this would not be a large advantage because the export parent code will be relatively small.
- Integrations plugins are cleaned up every time. No resource or memory leaks this way.
- Loading integration plugins every time takes acceptable time for every command we do. [Update 2020-05: this is no longer true with additions of mutations]

Advantages of direct calls
- Can pass callbacks, for example handle progress updates in the service.
- Easy passing/return of data and logs. Simple streaming data back from export-onboard-data.

Previously, when we did not have mutations using processes was better, with addition of mutations, it would be better to switch to direct calls and keep integration always running. Since starting them every time adds 170ms latency, which is bad for interactive user driven actions.

### Users from sourcecode integrations

#### User records from integration system

Normally sourcecode systems make it possible to export the list of users. We save those users in sourcecode.User directly from integration code.

For example in github, we create a user for every user in github organization. In further processing, when we process pull requests, comments and similar data, we also send newly encountered users. This would include old organization members, commits in public repos by others and bots, which are not included in org member list.

```
u = &sourcecode.User{}
u.ID = hash.Values("User", customerID, refType, refID)
u.RefType = "github"
u.RefID = githubUserID
u.Name = githubUserName
u.Email = ""
u.AssociatedRefID = nil
```

These objects contain integration user id as RefID. AssociatedRefID is not set for those users. We also do not set the email field, but in case it's possible to get the emails we will create additional objects as described in the next section.

#### User account emails from integration system

We also want to link emails used in commits and integration user accounts. These could be available in different ways.

For example, in gitlab enterprise it is possible to the email list for each user. In that case we pass a special integration commitusers.CommitUser struct from integration into agent.

```
type CommitUser struct {
	CustomerID string
	Email      string
	Name       string
	SourceID   string
}
```

In case of github, the emails are not available for retrieval based on user, but each commit in github api has account id and email, which we could use to link those.

We also send the same internal CommitUser struct to agent in this case.

To pass those objects to agent from integration, we write those into a session named sourcecode.CommitUser defined in commitusers.TableName constant. 

We take internal CommitUser struct and transform them into sourcecode.User written into sourcecode.CommitUser file.

This is done in cmdexport/process/users.go. It also checks that only one record per key is sent. key := email + "@@@" + sourceID

The resulting sourcecode.User object saved in sourcecode.CommitUser looks like this.

```
u = &sourcecode.User{}
u.ID = hash.Values("User", obj.CustomerID, email, "git", sourceID)
u.RefType = "git"
u.RefID = hash.Values(customerID, email)
u.Name = name
u.Email = email
u.AssociatedRefID = integrationUserID
```

#### User account emails from git

In addition, we create user records based on git data in exportrepo/riprsc. This is done in repo processing. It is similar to above case, but AssociatedRefID is empty.

```
u = &sourcecode.User{}
u.ID = hash.Values("User", obj.CustomerID, email, "git")
u.RefType = "git"
u.RefID = hash.Values(customerID, email)
u.Name = name
u.Email = email
u.AssociatedRefID = ""
```

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

