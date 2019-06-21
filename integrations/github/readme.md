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
````

### Other

Create auth token

https://help.github.com/en/articles/creating-a-personal-access-token-for-the-command-line
