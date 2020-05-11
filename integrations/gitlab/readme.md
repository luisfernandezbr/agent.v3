## Tested versions

- GitLab Enterprise Edition 12.2.4

## GitLab API Docs
- [Create auth token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)
- https://docs.gitlab.com/ee/api/
- https://docs.gitlab.com/ee/api/graphql/

- In order to get emails, the token needs to have Administrator privileges.

## [All exported data](./_docs/exported_data.md)

## API call examples

```
curl -H "Private-Token: $PP_GITLAB_TOKEN" http://lgitlab.com:8084/api/v4/user
```

## Development commands

```
Minimal required args
go run . export --agent-config-json='{"customer_id":"c1"}' --integrations-json='[{"name":"gitlab", "config":{"url":"https://YOUR_GITLAB_DOMAIN:PORT", "api_token":"XXX"}}]'
```

```
All args
go run . export --agent-config-json='{"customer_id":"c1", "skip_git":false}' --integrations-json='[{"name":"gitlab", "config":{"url":"https://YOUR_GITLAB_DOMAIN:PORT", "api_token":"XXX", "excluded_repos":[],"only_git":false,repos:["pinpt/test_repo"], "stop_after_n":1}}]'
```

```
URL      string `json:"url"`
APIKey string `json:"api_key"`

// ExcludedRepos are the repos to exclude from processing. This is based on github repo id.
ExcludedRepos []string `json:"excluded_repos"`
OnlyGit       bool     `json:"only_git"`

// Repos specifies the repos to export. By default all repos are exported not including the ones from ExcludedRepos. This option overrides this.
// Use gitlab nameWithOwner for this field.
// Example: user1/repo1
Repos []string `json:"repos"`

// StopAfterN stops exporting after N number of repos for testing and dev purposes
StopAfterN int `json:"stop_after_n"`
```    

## Design notes
We are mostly using REST API as GraphQL is often missing the data we need. We are only using GraphQL in ReposOnboardPageGraphQL which allows to save 1 request per object. Could be better to switch that to REST as well for consistency.

## Details on specific objects

### Missing data
- Is not possible to add reviews URLs.

### Exporting users

We fetch all users from using /users and for each one we call /users/:id/emails to get all user emails.

This is only available for a hosted version. We could not find a way to get user emails in cloud version.

- Direct link from commits to user ids is not available. See multiple issues in their bug tracker. [Example](https://gitlab.com/gitlab-org/gitlab-foss/issues/52106/)
- [User Emails API](https://docs.gitlab.com/ee/api/users.html#list-emails-for-user)
- There are issues with their user emails endpoint not returning users. [53618](https://gitlab.com/gitlab-org/gitlab-foss/issues/53618/)

### Pull Request Reviews
- For this integration we are fetching information from the pr discussions so I can know when the a reviewer approve a pr
- end point v4/projects/PROJECT_ID/merge_requests/PRIID/discussions

### Does updating pr node children update updated_at field on parent?

This is needed so that incremental export does not have to get call for pr comments and reviews on every single pr and other similar cases.

testing different cases

- create pr
    - created_at 2019-12-12T16:09:36.575Z
    - updated_at 2019-12-12T16:09:36.575Z

- update title
    - created_at 2019-12-12T16:09:36.575Z,
    - updated_at 2019-12-12T16:12:37.871Z, (change)

- create a comment on pr
    - created_at 2019-12-12T16:09:36.575Z,
    - updated_at 2019-12-12T16:13:42.957Z, (change)

- edit the comment on pr
    - created_at 2019-12-12T16:09:36.575Z,
    - updated_at 2019-12-12T16:16:05.575Z, (change)

## Work Issue Comments and Changelog

- The API used for this is the projects/project:id/issues/issue:id/discussions.json which returns a `note` object
- If the `note` is a _system note_, this it becomes a change, else it's a user comment

## Work Sprint

- The API used for this is the /projects/project:id/milestones


## Notes

The following API's are shared between the `WORK` and `SOURCECODE` integration.
  - sourcecode.User and work.User
  - sourcecode.Repo and work.Project

