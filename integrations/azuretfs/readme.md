## Azure DevOps and TFS integration

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

Most of the api is called asynchronously with the util functions:
```
sender := objsender.NewNotIncremental(s.agent, work.SprintModelName.String())
items, done := api.AsyncProcess("sprints", s.logger, func(model datamodel.Model) {
    if err := sender.Send(model); err != nil {
        // log error
    }
})
err := s.api.FetchSprints(projid, items)
close(items)
<-done
sender.Done()
```
The `api.AsyncProcess(name, logger, func(model))` returns a _items_ channel and a _done_ channel. The _items_ channel will be populated with the items from the API response converted into a  datamodel object. The callback is used to process each of these items. In this case, we use it to send them back to the agent one by one.

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

All the api related code is in the `api/` folder. The _sourcecode_ files are prefixed with `src_` and the _work_ files are prefixed with `work_`, the common files are prefixed with `common_`.

Most of the public functions have a private equivalent, this is for easy testing. For example, `FetchProjects()` calls `fetchProjects`. The public version of this function takes a datamodel channel but does not return anything, only an error. The private function returns the raw response in array format, we can use this in case we need to see what the response from the server is when we're testing.

This is also useful when we do async API calls; where the public function calls the private function asynchronously. 

For example in `FetchChangelogs`, where the public function takes a project id and the private does not. The public function calls the private function as many times as project ids, async.



