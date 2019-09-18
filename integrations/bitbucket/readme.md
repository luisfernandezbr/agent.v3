## Tested versions

- Bitbucket.com (2019-08-21)

## GitLab API Docs
- [Create auth token](https://confluence.atlassian.com/bitbucketserver/personal-access-tokens-939515499.html)
- https://developer.atlassian.com/bitbucket/api/2/reference/resource/

## TODO


## API call examples

The password can be an app password (https://confluence.atlassian.com/bitbucket/app-passwords-828781300.html)

```
curl --user USER:PASSWORD https://api.bitbucket.org/2.0/user
```

## Development commands

```
Minimal required args
go run . export --agent-config-json='{"customer_id":"c1"}' --integrations-json='[{"name":"bitbucket", "config":{"url":"https://api.bitbucket.org", "user":"XXX","password":"YYY"}}]'
```

```
All args
go run . export --agent-config-json='{"customer_id":"c1", "skip_git":false}' --integrations-json='[{"name":"bitbucket", "config":{"url":"https://api.bitbucket.org", "user":"XXX","password":"YYY", "excluded_repos":[],"only_git":false,repos:["pinpt/test_repo"], "stop_after_n":1}}]'
```

```
URL      string `json:"url"`
APIToken string `json:"apitoken"`

// ExcludedRepos are the repos to exclude from processing. This is based on github repo id.
ExcludedRepos []string `json:"excluded_repos"`
OnlyGit       bool     `json:"only_git"`

// Repos specifies the repos to export. By default all repos are exported not including the ones from ExcludedRepos. This option overrides this.
// Use bitbucket nameWithOwner for this field.
// Example: user1/repo1
Repos []string `json:"repos"`

// StopAfterN stops exporting after N number of repos for testing and dev purposes
StopAfterN int `json:"stop_after_n"`
```


## Onboard Users
- The account_id field will be used as the RefID which is a unique identifier across all atlassian(https://developer.atlassian.com/cloud/bitbucket/bitbucket-api-changes-gdpr/#introducing-atlassian-account-id-and-nicknames)
- It is not possible to get emails from users `/teams/:name/members`
- It is not possible to get usernames from users `/teams/:name/members`

## TODO
- There is a pull request state called SUPERSEDED, not sure how interpret this kind of PR

## Notes
-- It's not possible to get URL for reviews