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
id
name
avatarUrl
login
url
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
id
nameWithOwner
url	
description		
primaryLanguage {
    name
}
isFork
isArchived
```

### Commits

```
oid
author {
    name
    email
    user {
        login
    }
}
committer {
    name
    email
    user {
        login
    }
}
}	
```

## Pull Requests

```
updatedAt
id
number
repository {
	id
	nameWithOwner
}
headRefName
title
bodyHTML
url
createdAt
mergedAt
closedAt
state
draft: isDraft
locked
author { login }
mergedBy { login }
mergeCommit { oid }
commits(last: 1) {
	nodes {
		commit {
			oid
		}
	}
}
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

## Pull Requests Commit
```
commit {
    oid
    message
    url
    additions
    deletions
    author {
        email
    }
    committer {
        email
    }
    authoredDate
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
bodyHTML
createdAt
author {
	login
}
```

## User

```
{
	__typename
	... on User {
		id
		name
		avatarUrl
		login
		url		
	}
	... on Bot {
		id
		avatarUrl
		login
		url		
	}
}
```

## Pull Request Timeline
```
{
... on PullRequestReview {
    __typename
    id
    url
    state
    createdAt
    author User
}
... on ReviewRequestedEvent {
    __typename
    id
    createdAt
    requestedReviewer User
}
... on ReviewRequestRemovedEvent {
    __typename
    id
    createdAt
    requestedReviewer User
}
... on AssignedEvent {
    __typename
    id
    createdAt
    assignee User
}
... on UnassignedEvent {
    __typename
    id
    createdAt
    assignee User
}
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
