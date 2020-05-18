## Exported data

Integrations differ in data that they export. Right now we have 3 different integration types, sourcecode, work and calendar.

See an example of data exported for [github integration here](../integrations/github/_docs/exported_data.md).

Sourcecode integrations also process git repositories directly after cloning them using git.

## Data processing for git repos

Sourcecode integrations clone git repositories to the machine where agent is running. Each commit in repository is processed, code statistics are extracted and send to the server. We do not send the actual code after processing with agent. 

We send metadata for every commit, please see the objects below for details.

See the matching sourcecode in `./slimrippy/exportrepo`

```
sourcecode.Commit
    RefID
    RefType
    CustomerID
    RepoID
    Sha
    Message
    URL
    AuthorRefID
    CommitterRefID
    Identifier
    CreatedDate

    Author and Committer
        Email
		Name

sourcecode.Branch
    RefID
    Name
    URL
    RefType
    CustomerID
    Default
    Merged
    MergeCommitSha
    MergeCommitID
    BranchedFromCommitShas
    BranchedFromCommitIds
    CommitShas
    CommitIds
    FirstCommitSha
    FirstCommitID
    BehindDefaultCount
    AheadDefaultCount
    RepoID

sourcecode.PullRequestBranch
    PullRequestID
    RefID
    Name
    URL
    RefType
    CustomerID
    Default
    Merged
    MergeCommitSha
    MergeCommitID
    BranchedFromCommitShas
    BranchedFromCommitIds
    CommitShas
    CommitIds
    BehindDefaultCount
    AheadDefaultCount
    RepoID
```