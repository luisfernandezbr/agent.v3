## Azure DevOps and TFS integration

### Contents

- [Exported data for sourcecode](./_docs/exported_data_code.md)
- [Exported data for work](./_docs/exported_data_work.md)
- [TFS API Docs](https://docs.microsoft.com/en-us/azure/devops/integrate/previous-apis/overview?view=azure-devops-2019)
- [Azure API Docs](https://docs.microsoft.com/en-us/rest/api/azure/devops/?view=azure-devops-rest-5.1)

### Export command

Create an `export.json` file in the root of the agent repo with the following

(modify for your env)
```
[
    {
        "name": "azuretfs",
        "config": {
            "reftype": "azure", // or "tfs"
            "type": "",         // can be SOURCECODE, WORK, or empty for both
            "concurrency": 10,
            "credentials": {
                "url": "https://dev.azure.com", // url to the server
                "organization": "",             // org name
                "api_key": "",                  // api key

                "username: "", // needed for git clone
                "password": "" // needed for git clone
            }
        }
    }
]
```
Then run:
```
 go run main.go export \
    --agent-config-json='{"customer_id":"CUSTOMER_ID"}' \ 
    --integrations-file=./export.json \
    --pinpoint-root=$HOME/.pinpoint/next-azure
```

### Integration

#### Export

Depending on the integration type, sourcecode or work, it calls `exportCode()` or `exportWork()`. These are located in `code_export.go` and `work_export.go`.

Some of the API functions take an `objsender2.Session` object as a parameter and send the objects to the agent, while others return the objects and do it in the export.go, this all depends on the complexity of the fetch.

Whenever ever possible, the `azureapi.Async` is used to speed things up by calling API's asynchronously. For example in [Pull Requests](./api/scr_pull_requests.go)  where the comments, commits, and reviews are fetch per PR. 

#### ValidateConfig

For validation, the integration only calls the `FetchAllRepos` API to make sure it does not fail and no errors are returned.

#### OnboardExport

The functions for the OnboardExport are located in onboard.go

### Missing datamodel object properties

```
sourcecode.PullRequest:
    - UpdatedDate
sourcecode.PullRequestReview
    - CreatedDate
sourcecode.PullRequestComment
    - URL
sourcecode.Repo
    - Description
    - Language
work.Issue
    - CustomFields
    - ParentID
work.Project
    - Category
work.User
    - Email
work.Sprints
    - CompletedDate (using EndedDate)
    - Goal
```

### CommitUser

This object is missing. The users we get from the commits API are the same as the git blame users. The only information we get is the person's name, email, and date of commit. To create this object we would need to match it with the team users (from the team user's API), and there is no real way to do this since the team users don't have an email associated with them.

### Incremental

The only API's that support incremental export in this integration are the `FetchWorkItems` and `FetchChangelogs`. We also added incremental export to the pull request API's by manually filtering the responses by date, but the API's don't
support this. The rest of the API's _do not_ have incremental export support.

### API file structure

All the API related code is in the `api/` folder. The _sourcecode_ files are prefixed with `src_` and the _work_ files are prefixed with `work_`, the common files are prefixed with `common_`.
