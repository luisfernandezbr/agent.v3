# Self hosted Jira

https://docs.atlassian.com/software/jira/docs/api/REST/8.3.0/

JIRA 8.3.0

# Authentication
OAuth authentication is not supported in Jira Server, only Jira Cloud. Here is the ticken on the current status:
https://jira.atlassian.com/browse/JRASERVER-43171


# Example request

```
curl -u user:pass -X GET -H "Content-Type: application/json" 'https://localhost:8443/rest/api/2/project' --insecure | jq . | less
```

# TODO

- Project.active field is always true, is it needed?
- CustomField.key does not exist for hosted jira
- Project.url TODO