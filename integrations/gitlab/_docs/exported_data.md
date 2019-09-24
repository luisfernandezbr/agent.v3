## All exported objects and fields for Gitlab

See the description of common exported data based on [git repos here](../../../_docs/exported_data.md).

### Gitlab [readme](../readme.md)

## Common

### Groups

#### List groups

https://docs.gitlab.com/ee/api/groups.html#list-groups

#### Fields used

```
name
```

### Onboard Group Repos

#### List onboard group repos

https://docs.gitlab.com/ee/api/graphql/reference/

#### Fields used

```
		group(fullPath:"` + groupName + `") {
			projects(` + projectParams + `) {
				edges {
					cursor
					node {
						id
						fullPath
						description
						createdAt
						repository {
							tree {
								lastCommit {
									author {
										avatarUrl
									}
								}
							}
						}
					}
				}
			}
		}
```

#### List group repos

https://docs.gitlab.com/ee/api/groups.html#list-a-groups-projects

#### Fields used

```
id
path_with_namespace
default_branch
```

#### List group repos with details

https://docs.gitlab.com/ee/api/groups.html#list-a-groups-projects

#### Fields used

```
created_at
last_activity_at
id
path_with_namespace
description
web_url
```

### Repo pull requests

#### List repo pull requests

https://docs.gitlab.com/ee/api/merge_requests.html#list-project-merge-requests

#### Fields used

```
id
iid
updated_at
created_at
closed_at
merged_at
source_branch
title
description
web_url
state
author{
    username
}
closed_by{
    username
}
merged_by{
    username
}
merge_commit_sha
```

### Pull request comments

#### List pull request comments

https://docs.gitlab.com/ee/api/notes.html#list-all-merge-request-notes

#### Fields used

```
id
author{
    username
}
body
updated_at
created_at
```

### Pull request commits

#### List pull request commits

https://docs.gitlab.com/ee/api/merge_requests.html#get-single-mr-commits

#### Fields used

```
id
message
created_at
author_email
committer_email
```

### Pull request reviews

#### List pull request approvals

https://docs.gitlab.com/ee/api/merge_request_approvals.html#merge-request-level-mr-approvals

#### Fields used

```
id
approved_by{
    user{
        username
    }
}
suggested_approvers{
    user{
        username
    }
}
approvers{
    user{
        username
    }
}
created_at
updated_at
```

### Approved dates

#### List pull request discussions

https://docs.gitlab.com/ee/api/discussions.html#list-project-merge-request-discussion-items

#### Fields used

```
notes{
    author{
        username
    }
    body
    created_at
}
```

### Commit stats

#### List commit details

https://docs.gitlab.com/ee/api/commits.html#get-a-single-commit

#### Fields used

```
stats{
    additions
    deletions
}
```

## Cloud specific

### Project Users

#### List project users

https://docs.gitlab.com/ee/api/projects.html#get-project-users

#### Fields used

```
id
name
username
avatar_url
```

## On-premise specific

### Users

#### List users

https://docs.gitlab.com/ee/api/users.html

#### Fields used

```
username
email
name
id
avatar_url
```
