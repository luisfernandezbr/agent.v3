## Jira integration

## Table of contents

- [Jira Cloud](#jira-cloud)
- [Jira Server](#jira-server)
- [Exported data](./_docs/exported_data.md)

We have separate binaries for jira cloud and jira server.

## Jira Cloud

- https://developer.atlassian.com/cloud/jira/platform/rest/v3/
- Cloud REST v3

### Authentication

Supports basic auth and OAuth 2

- [Jira cloud basic auth](https://developer.atlassian.com/cloud/jira/platform/jira-rest-api-basic-authentication/)
- [Jira cloud OAuth 2](https://developer.atlassian.com/cloud/jira/platform/oauth-2-authorization-code-grants-3lo-for-apps/)

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
