- [Build from source](./_docs/build.md)
- [Architecture](./_docs/architecture.md)
- [Development workflow](./_docs/dev_workflow.md)
- [Exported data](./_docs/exported_data.md)

### Required git version

| Version                             | Notes  
| -------------                       | -------- 
| 2.20.1             | Default macos version. Works fine.
| 2.13               | Released on 2017-05. Introduced clone --no-tags flag. Should work.
| <2.13              | We do not support older versions.

### Other features

#### Running additional integrations in export

You can add an extra configration line to config after running enroll. They will be run when export is requested in service-run. See example below.

```
{
.... existing fields,
"extra_integrations": [{"name":"mock", "config":{"k":"v"}}]
}
```