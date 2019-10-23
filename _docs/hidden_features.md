### Hidden features

#### Running additional integrations in export

You can add an extra configration line to config after running enroll. They will be run when export is requested in service-run. See example below.

```
{
.... existing fields,
"extra_integrations": [{"name":"mock", "config":{"k":"v"}}]
}
```