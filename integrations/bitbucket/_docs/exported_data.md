## All exported objects and fields for Bitbucket

See the description of common exported data based on [git repos here](../../../_docs/exported_data.md).

## Common

### Groups

#### List groups

https://developer.atlassian.com/bitbucket/api/2/reference/resource/teams

#### Fields used

```
username
```

### Onboard Group Repos

#### List onboard group repos

https://developer.atlassian.com/bitbucket/api/2/reference/resource/teams/%7Busername%7D/repositories

#### Fields used

```
uuid
full_name
description
language
created_on
```

#### List group repos

https://developer.atlassian.com/bitbucket/api/2/reference/resource/teams/%7Busername%7D/repositories

#### Fields used

```
uuid
full_name
mainbranch{
    name
}
```

#### List group repos with details

https://docs.gitlab.com/ee/api/groups.html#list-a-groups-projects

#### Fields used

```
created_on
updated_on
uuid
full_name
description
links{
    html{
        href
    }
}
```

### Repo pull requests and reviews/approvals

#### List repo pull requests

https://developer.atlassian.com/bitbucket/api/2/reference/resource/repositories/%7Busername%7D/%7Brepo_slug%7D/pullrequests#get

#### Fields used

```
id
source{
    branch{
        name
    }
}
title
description
links{
    html{
        href
    }
}
created_on
updated_on
state
closed_by{
    account_id
}
merge_commit{
    hash
}
author{
    account_id
}
participants{
    role
    approved
    user{
        account_id
    }
}
```

### Pull request comments

#### List pull request comments

https://developer.atlassian.com/bitbucket/api/2/reference/resource/repositories/%7Busername%7D/%7Brepo_slug%7D/pullrequests/%7Bpull_request_id%7D/comments#get

#### Fields used

```
id
links{
    html{
        href
    }
}
updated_on
created_on
content{
    raw
}
user{
    account_id
}
```

### Pull request commits

#### List pull request commits

https://developer.atlassian.com/bitbucket/api/2/reference/resource/repositories/%7Busername%7D/%7Brepo_slug%7D/pullrequests/%7Bpull_request_id%7D/commits

#### Fields used

```
hash
message
date
author{
    raw
}
```

### Commit stats

### Users and onboard users

#### List team users

https://developer.atlassian.com/bitbucket/api/2/reference/resource/teams/%7Busername%7D/members

#### Fields used

```
display_name
links{
    avatar{
        href
    }
}
account_id
```

### Commit users

#### List repo commits

https://developer.atlassian.com/bitbucket/api/2/reference/resource/repositories/%7Busername%7D/%7Brepo_slug%7D/commits#get

#### Fields used

```
author{
    raw
    user{
        display_name
        account_id
    }
}
date
```
