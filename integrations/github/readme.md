## Support continuing interrupted initial export, as well as incremental exports

We need to store both cursor (opaque string) and last processed to support both.

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

In current implementation we do not save cursor to disk in partial exports, since we do not safely write exported objects to file. But we do keep it in memory while exporting.

Need cursor because github graphql api uses non-date cursor when sorting by updated_at.

Have to do incrementals backwards, since api does not allow filtering after certain updated_at date. Github allowed since parameter in v3 rest api, but not in v4 graphql.

But they do allow since parameter for issues, since we also need pull requests, needed something more generic.

### Other

Create auth token

https://help.github.com/en/articles/creating-a-personal-access-token-for-the-command-line
