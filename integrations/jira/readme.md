## Contents

- [Jira Cloud](#jira-cloud)
- [Jira Server](#jira-server)
- [Exported data](./_docs/exported_data.md)

## Jira integration

We have separate binaries for jira cloud and jira server.

## Jira Cloud

- https://developer.atlassian.com/cloud/jira/platform/rest/v3/
- Cloud REST v3

### Authentication

Supports basic auth and OAuth 2

- [Jira cloud basic auth](https://developer.atlassian.com/cloud/jira/platform/jira-rest-api-basic-authentication/)
- [Jira cloud OAuth 2](https://developer.atlassian.com/cloud/jira/platform/oauth-2-authorization-code-grants-3lo-for-apps/)

### Required permissions

In general jira token does not have separate levels of permissions. But the users that this token belongs to can have different visibility and security settings. The user used for export should be able to see all issues and related data that we want to texport.

Permission is Browse Project for each. If the project has issues with different Security levels, the user should have the valid permission to see those.

[General jira cloud docs on managing permissions](https://confluence.atlassian.com/adminjiracloud/managing-project-permissions-776636362.html)

### Example request

```
curl -u user@example.com:API_TOKEN 'https://pinpt-hq.atlassian.net/rest/api/3/search'
```

## Jira Server

- https://docs.atlassian.com/software/jira/docs/api/REST/8.3.0/
- JIRA 8.3.0

### Authentication
OAuth authentication is not supported in Jira Server, only Jira Cloud. Here is the ticket on the current status:

https://jira.atlassian.com/browse/JRASERVER-43171

### Example request

```
curl -u user:pass -X GET -H "Content-Type: application/json" 'https://localhost:8443/rest/api/2/project' --insecure | jq . | less
```

### Development

#### Encode params for jira search

https://play.golang.org/

```
package main

import (
	"fmt"
	"net/url"
)

func main() {
	projectJiraID := "10"
	jql := `project="` + projectJiraID + `"`
	params := url.Values{}
	params.Set("jql", jql)
	fmt.Println(params.Encode())
}
```