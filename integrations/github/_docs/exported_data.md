## All exported objects and fields for Github

See the description of common exported data based on [git repos here](../../../_docs/exported_data.md).

### REST API, only for used Github Enterprise

https://developer.github.com/enterprise/2.15/v3/orgs/#list-all-organizations

```
url
/organizations

fields
login
```

### GraphQL API

https://developer.github.com/v4/

### Organizations

```
login
```

## Users of organization and authors of all commits 

```
name
email
avatarUrl
login
```

## Repositories

```
id
createdAt
updatedAt

nameWithOwner
defaultBranchRef {
    name
}
updatedAt
url	
description		
isArchived	
primaryLanguage {
    name
}
isFork
isArchived
```

### Commits

```
oid
message
url
additions
deletions
authoredDate
committedDate
author
committer
```

## Pull Requests

```
updatedAt
id
repository { id }
headRefName
title
bodyText
url
createdAt
mergedAt
closedAt
state
author { login }						
mergedBy { login }
mergeCommit { oid }
comments {
    totalCount
}
reviews {
    totalCount
}
closedEvents: timelineItems (last:1 itemTypes:CLOSED_EVENT){
    nodes {
        ... on ClosedEvent {
            actor {
                login
            }
        }
    }
}
```

## Pull Request Comments
```
updatedAt
id
url
pullRequest {
    id
}
repository {
    id
}						
bodyText
createdAt
author {
    login
}
```

## Pull Request Reviews

```
updatedAt
id
url
pullRequest {
    id
}
repository {
    id
}
state
createdAt
author {
    login
}
```