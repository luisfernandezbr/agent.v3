## Exported data

Integrations differ in data that they export. Right now we have 2 different types of integrations, sourcecode and work.

See an example of data exported for [github integration here](../integrations/github/_docs/exported_data.md).

Sourcecode integrations also process git repositories directly after cloning them using git.

## Data processing for git repos

Sourcecode integrations clone full git repositories, and send content of all commits to the backend, with some exceptions for performance reasons.

Agent also calculated some additional statistic and similar data based on this, which is sent to the server as well.