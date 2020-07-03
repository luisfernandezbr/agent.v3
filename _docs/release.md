## Release a new on-premise agent version

- start by checking out master and making sure you don't have any changes in git repo
- get a new verstion, it should look somewhat like v0.0.99
- build and upload release to S3

```go run ./cmd/agent-dev build --upload --version="VERSION"```

- in github interface create a new release
- upload the github release zips from dist folder
- save new release
- set the on-premise agents to update using mongodb

```
// first get the current date
go run ./cmd/agent-dev date
db.ops_cloudagent_control.remove({})
// paste the current date in the command below
db.ops_cloudagent_control.insertOne({"on_premise_update_requested_date": {"epoch":1585849950149,"offset":120,"rfc3339":"2020-04-02T19:52:30.149+02:00"}})
```

## Release a new docker on-premise build
- this happens at the same time as cloud agent release, see next section

## Release a new cloud agent version
- have your pr merged
- approve the release to stable in codefresh
- set the cloud agents to update using mongodb

```
// first get the current date
go run ./cmd/agent-dev date
db.ops_cloudagent_control.remove({})
// paste the current date in the command below
db.ops_cloudagent_control.insertOne({"update_requested_date": {"epoch":1585849950149,"offset":120,"rfc3339":"2020-04-02T19:52:30.149+02:00"}})
```