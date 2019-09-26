## All exported objects and fields for Jira

### Projects

```
self
id
key
name
description
category
    id 
    name
```

### Users

```
accountId - only for cloud jira
self
name
key
emailAddress
avatarUrls
displayName
active
timeZone
groups
active
```

### Fields

```
id
key - only for cloud jira
name
```

### Project issues

```
id
key
fields
    summary
    duedate
    created
    updated
    priority
    issuetype
    status
    resolution
    creator
    reporter
    assignee
    labels
changelog
    histories
        id
        author
        created
        items
            field
            fieldType
            from
            fromString
            toString
            tmpFromAccountId
            tmpToAccountId
```