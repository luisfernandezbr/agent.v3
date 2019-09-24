## Exported data

Integrations differ in data that they export. Right now we have 2 different types of integrations, sourcecode and work.

See an example of data exported for [github integration here](../integrations/github/_docs/exported_data.md).

Sourcecode integrations also process git repositories directly after cloning them using git.

## Data processing for git repos

Sourcecode integrations clone git repositories to the machine where agent is running. Each commit in repository is processed, code statistics are extracted and send to the server. We do not send the actual code after processing with agent. 

But we do send metadata for every commit, file and line. For example, we send flags for each line to know if it's code, comment or empty line, as well as in which commit it was modified, author, dates and similar.

```
sourcecode.Commit
    Metadata
        Sha
        Message
        FilesChanged
        AuthorEmailHash
        CommitterEmailHash
        IsGpgSigned

    Counts
        Additions
        Deletions
        Loc
        Sloc
        Comments
        Blanks
        Size
        Complexity

    Files 
        Metadata for each file

        Filename
        Language
        Renamed
        Additions
        Deletions
        License
        Size
        Loc
        Sloc
        Comments
        Blanks
        Complexity

sourcecode.Blame
    This is information about each file changed in commit. Somewhat similar to Files above, but also included information about the lines in the file.
    License
    Size
    Loc
    Sloc
    Blanks
    Comments
    Filename
    Language
    Sha
    Complexity
    Lines
        LastCommit
        AuthorEmailHash
        Date
        IsComment
        IsCode
        IsBlank

sourcecode.Branch
    Name
    CommitShas
    BehindDefaultCount
    AheadDefaultCount
```