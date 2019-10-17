## Version notes

| Version                             | Notes  
| -------------                       | -------- 
| GitHub.com (2019-10-09)             | Tested
| Github Enterprise 2.17.9            | Tested. Client 1 was using this version on 2019-10-09.
| Github Enterprise 2.16              | Should work, not tested. Has PullRequest.timelineItems. But require preview accept header set. We are setting it.
| GitHub Enterprise 2.15.9            | Not supported. Client 1 was using this version on 2019-08-21. Does not have PullRequest.timelineItems -> can't get PullRequest.ClosedByRefID. Will break now, since we aren't checking versions.

You can see GitHub Enterprise version number in the webapp by hovering GitHub Octocat logo in the footer. It should be shown in the tooltip.

## GitHub API Docs
- https://developer.github.com/v4/
- https://developer.github.com/enterprise/2.15/

## [All exported data](./_docs/exported_data.md)

## Authentication

It supports both auth tokens created manually and tokens created using OAuth flow.

In both cases pass the token as apitoken.

- [Create auth token](https://help.github.com/en/articles/creating-a-personal-access-token-for-the-command-line)
- [OAuth docs](https://developer.github.com/v3/oauth_authorizations/)

Github OAuth tokens do not expire, so not need to use pinpoint token refresh api, which is different from jira.

### Token permissions required for github.com and enterprise

https://developer.github.com/apps/building-oauth-apps/understanding-scopes-for-oauth-apps/

The permissions are the same between them. But for github.com token also need to press "Enable SSO" button for organizations that have SSO.

- repo X (the options below are selected automatically)
    - repositories X
    - repo_deployment X
    - public_repo X
    - repoinvite X
- admin:org (only read access below)
    - read:org X
- user (specific options below)
    - read:user X
    - user:email X

## TODO
- sourcecode.PullRequest.ClosedByRefID not implemented for GitHub Enterprise 2.15.9. timelineItems is not available need to get it another way

## API call examples

```
curl -H "Authorization: bearer $PP_GITHUB_TOKEN" -X POST -d '{"query":"query { viewer { login }}"}' https://api.github.com/graphql
```

## Development commands

```
Minimal required args
go run . export --agent-config-json='{"customer_id":"c1"}' --integrations-json='[{"name":"github", "config":{"url":"https://api.github.com", "apitoken":"XXX"}}]'
```

## Datamodel notes
github.PullRequestComment does not include comments created from review, these go to github.PullRequestReview. We do not currently store the text of those.

## How to support incremental exports?
Since graphql doesn't support since parameter on all objects, we iterate backwards using updated at timestamp, when we get to the object which was updated before last run we stop.

It was possible in github v3 rest api, but in v4 graphql it's only supported for issues.

There is a possible issue with iterating backwards using updated_at. If an object is updated while we are iterating, it could move from the page we haven't seen yet to the end we already iterated. It's not a big deal, since it would be picked up on next incremental export.

Possible performance optimization is to limit the number of records returned to 1 for first incremental request, to quickly see if there are any records. Not worth doing now.

## Performance

It takes ~?m on pinpoint organization for initial export and ~?m for incremental immediately after. Using ?% of github.com hourly quota.

## How to iterate on initial export and possibly continue on interruption?

If we want to support interrupting and continuing on intial export we need to store the cursor (opaque string) as well.

We do not store it on disk in the current implementation, since we don't write exported objects to file properly when interrupted, but this can be fixed.

We need cursor because github graphql api uses non-date cursor when sorting by updated_at.

It would be possible to iterate backwards the same way for incremental and initial export, but due to the possible issue documented above, we iterate forwards on initial export.

Here is the algo:

```
if no lastProcessed
    if no cursor
        from start saving cursor every page
    if cursor
        from cursor saving cursor every page
if lastProcessed
    from end backwards
    stop when <= lastProcessed
    save new lastProcessed
```

### Does updating node children update updated_at field on parent?

This is needed so that incremental export does not have to get call for pr comments and reviews on every single pr and other similar cases.

testing different cases

- create pr, note updatedAt date, 2019-06-24T16:07:35Z
- create a comment on pr, see pr updated_at date, 2019-06-24T16:11:20Z (updated)
- edit the comment on pr, see pr updated_at date, 2019-06-24T16:12:19Z (updated)
- create review on pr, see pr updated_at date, 2019-06-24T17:45:30Z
- edit review on pr (resolve conversation), see pr updated_at date, 2019-06-24T17:45:30Z (does not change)
- update comment on pr, date: 2019-06-24T17:52:23Z

So when fetching pr comments we can only fetch comments for prs with new updated_at date.

Adding review updates pull request updated_at, but not for all changes, for example resolve conversation does not. (We will not refetch pr review on resolve for now)

When not using updated_at filter it is sorted by created_at by default. So the objects that do not have updated_at filter have to be always fully refetched, for example pr comment and pr review.

- testing if updating comment on pr sets updated_at on repo (no it does not)

In general this needs to be tested on case by case basic. This relies on github private api implementation details. But don't know of any better way to avoid re-fetching all data on incremental.

## Exporting users

We first export all users belonging to organization. The github api does not return email in that case, so we skip that.

Afterwards, when we export pull requests and related users, we check all authors using login if we have already exported a user with this login we skip it, if we haven't yet, we export this users, setting (organization) member field to false.

We also retrieve commits from the api to get the link from email to github login.

When commit has a login, we first do the same process of sending this user as for pull requests (only if we haven't sent the user already).

As a second step, for both author and committer, we send a entry with name, email and if it exists a github login link.

Agent checks if we have already sent it behind the scenes, and if not creates a user with github and git ref types.
