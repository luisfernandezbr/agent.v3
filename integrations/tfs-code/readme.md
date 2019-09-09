## TFS git integration

## Export command

```
Integrations JSON:
{
    "name":"tfs-code",
    "config": {
        "api_key":   API_KEY,       // string, required
        "url":       URL_ENDPOINT,  // string, required
        "username":  USERNAME,      // string, required, for git clone
        "password":  PASSWORD,      // string, required, for git clone
        "excluded_repo_ids": ...    // array, optional, list of repo ids to _not_ clone
        "collection": ...           // string, optional, name of collection to use
    }
}
----------
go run . export \
    --agent-config-json='{"customer_id":"customer_id"}' \
    --integrations-json='[{"name":"tfs-code", "config":{"api_key":API_KEY, "url":URL_ENDPOINT, "username":USERNAME, "password": PASSWORD, "excluded_repo_ids": IDS_ARRAY}}]' \
    --pinpoint-root=$HOME/.pinpoint/next-tfs-code
```

## Running tests

To run the tests you'll need to enable it with the `PP_TEST_TFS_CODE` flag set to "1", you'll also need the api key and the api url

```
PP_TEST_TFS_CODE_URL=https://api-url PP_TEST_TFS_CODE_APIKEY=1234567890 PP_TEST_TFS_CODE=1 go test github.com/pinpt/agent.next/integrations/tfs-code...
```

## API

- FetchRepos
    Calls `/_apis/git/repositories` and grabs all the repos from the collection.
    If the collection name is not passed in when running export, it will use the default collection `DefaultCollection`
    Once the api comes back with all the repos, it will filter out the `excluded_repo_ids`
    This API does not support incrementals, all repos will be fetched
    - `sourcecode.Repo` missing properties:
        - `Language`
        - `Description`

- FetchPullRequests
    Calls `_apis/git/repositories/{repo_id}/pullRequests`
    Returns a list of all the pull requests and pull request reviews objects:
    This API does not support incrementals, all pull requests will be fetched by the api and then filtered in code by date when running an incremental export
    - `sourcecode.PullRequest`
        There are three statuses for the pull requests, mapped this way
            - "completed" = sourcecode.PullRequestStatusMerged
            - "active"    = sourcecode.PullRequestStatusOpen
            - "abandoned" = sourcecode.PullRequestStatusClosed
    - `sourcecode.PullRequestReview`
        This object comes from the same api call as the pull request
        The `vote` property determines the state of the pull request, according to the [docs](https://docs.microsoft.com/en-us/azure/devops/integrate/previous-apis/git/pull-requests/reviewers?view=azure-devops-2019#add-a-reviewer): `-10 means "Rejected", -5 means "Waiting for author", 0 means "No response", 5 means "Approved with suggestions", and 10 means "Approved"`       

- FetchPullRequestComments
    Calls `_apis/git/repositories/{repo_id}/pullRequests/{pull_request_id}/threads`
    Returns the comments from each pull request, filtering out the automatic comments by looking for `commentType: "text"`
    This API does not support incrementals, all comments will be fetched by the api and then filtered in code by date when running an incremental export
    - `sourcecode.PullRequestComment`
        Missing URL
    
- FetchCommitUsers:
    Calls `_apis/git/repositories/{repo_id}/commits` to fetch the user information
    Pass in a map of user_id and users to make sure we don't have duplicates
    
## Incrementals
In incremental processing, we have special handling for pull request comments. We re-fetch all comments for pull requests which are open or were closed since last processing. All PullRequestReviews are re-fetched every time.

Commits (used for users) in incrementals are retrieved based on date (fromDate).

No incrementals for pull request commits.

## Missing data

Onboarding

```
Repos
language
last_commit.author.avatar_url
last_commit.author.email    

Users
emails
id !critical
username !critical
```

Export

```
Repo
language
description

Users
email

CommitUser
associated_ref_id !critical

PullRequest
All fields there

PullRequestComment
url

PullRequestReview
There is no history of reviews, only latest !critical
created_date
updated_date
url

```