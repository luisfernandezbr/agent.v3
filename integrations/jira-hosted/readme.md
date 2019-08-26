# Self hosted Jira

https://docs.atlassian.com/software/jira/docs/api/REST/8.3.0/

JIRA 8.3.0

# Example request

```
curl -u user:pass -X GET -H "Content-Type: application/json" 'https://localhost:8443/rest/api/2/project' --insecure | jq . | less
```

# TODO

- Project.active field is always true, is it needed?
- CustomField.key does not exist for hosted jira
- Project.url TODO