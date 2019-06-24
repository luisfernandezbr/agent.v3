## TODO

- pull_request is missing closed_by_id, closed_by_login (this fields are not available on PR and are somehow related to issue)

## How to support incremental exports?
Since graphql doesn't support since parameter on all objects, we iterate backwards using updated at timestamp, when we get to the object which was updated before last run we stop.

It was possible in github v3 rest api, but in v4 graphql it's only supported for issues.

There is a possible issue with iterating backwards using updated_at. If an object is updated while we are iterating, it could move from the page we haven't seen yet to the end we already iterated. It's not a big deal, since it would be picked up on next incremental export.

Possible performance optimization is to limit the number of records returned to 1 for first incremental request, to quickly see if there are any records. Not worth doing now.

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

This is needed so that incremental export does not have to get all data again.

testing different cases

create pr, note updatedAt date, 2019-06-24T16:07:35Z
create a comment on pr, see pr updated_at date, 2019-06-24T16:11:20Z (updated)
edit the comment on pr, see pr updated_at date, 2019-06-24T16:12:19Z (updated)
create review on pr, see pr updated_at date, 2019-06-24T17:45:30Z
edit review on pr (resolve conversation), see pr updated_at date, 2019-06-24T17:45:30Z (does not change)
update comment on pr, date: 2019-06-24T17:52:23Z

So when fetching pr comments we can only fetch comments for updated prs.

Adding review updates pull request, but not all changes change date, for example resolve conversation does not.

When not using updated_at filter it is sorted by created_at by default.

test if updating comment on pr sets updated_at on repo (no it does not)
so we need to check this on case by case basis, not all updates are propagated





### Other

Create auth token

https://help.github.com/en/articles/creating-a-personal-access-token-for-the-command-line
