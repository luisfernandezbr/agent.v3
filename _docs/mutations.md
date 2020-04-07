## Data mutations

### Performance

It takes 2.5-2.7s second for the mutation to execute if measuring from when even is received in agent operator till response is completed. Tested locally, should be faster on aws.

Of that time 0.6-0.8s is taken by getting updated issue from jira.

## Supported mutations

### Jira Cloud

#### Set issue title

```
{
    "integration_name": "jira",
    "system_type": "work",
    "action": "ISSUE_SET_TITLE",
    "data": {
        "ref_id": "TES-79",
        "title": "title 17:16"
    }
}
```

#### Set issue assignee

Pass empty string as user_ref_id to unset.

```
{
    "integration_name": "jira",
    "system_type": "work",
    "action": "ISSUE_SET_ASSIGNEE",
    "data": {
        "ref_id": "TES-79",
        "user_ref_id": "5c2a767a75b0e95216e8fa16"
    }
}
```

Integration exports the list of all possible users as work.User. This list should be filtered in webapp? by integration_id of that issue. TODO: pipeline should set integration_id field on all objects

#### Set issue priority

```
{
    "integration_name": "jira",
    "system_type": "work",
    "action": "ISSUE_SET_PRIORITY",
    "data": {
        "ref_id": "TES-79",
        "priority_ref_id": "10000"
    }
}
```

The list of possible priorities is available as work.IssuePriority. This list should be filtered in webapp? by integration_id of that issue. TODO: pipeline should set integration_id field on all objects

#### Set issue status

Before issuing this query you would need to get the list of possible status transitions and required fields using 'Get issue transitions' ISSUE_GET_TRANSITIONS.

```
{
    "integration_name": "jira",
    "system_type": "work",
    "action": "ISSUE_SET_STATUS",
    "data": {
        "ref_id": "TES-79",
        "transition_id": "41",
        "fields": {
            "resolution":"Hardware failure"
        }
    }
}
```

#### Get issue transitions

Returns the list of possible status transitions.

```
Request
{
    "integration_name": "jira",
    "system_type": "work",
    "action": "ISSUE_GET_TRANSITIONS",
    "data": {
        "ref_id": "TES-79"
    }
}
```

```
Response
[
{ "fields": null, "id": "11", "name": "To Do" },
{ "fields": null, "id": "21", "name": "In Progress" },
{    
	"fields": [
        {
        "allowed_values": [
            { "id": "10000", "name": "Done" },
            { "id": "10001", "name": "Won't Do" },
            { "id": "10200", "name": "Invalid" }
        ],
        "required": true,
        "id": "resolution",
        "name": "Resolution"
        }
    ],
    "id": "41",
    "name": "Closed"
}
]
```

#### Add comment to an issue

This only allows adding simple text comments without formatting.

```
{
    "integration_name": "jira",
    "system_type": "work",
    "action": "ISSUE_ADD_COMMENT",
    "data": {
        "ref_id": "TES-79",
        "body": "Content of a comment as text"
    }
}
```