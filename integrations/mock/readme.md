## Mock integration

### Export command

```
go run . export --agent-config-json='{"customer_id":"c1"}' --integrations-json='[{"name":"mock", "config":{"k":"v"}}]'
```

Windows Powershell
```
.\agent-next.exe export --% --agent-config-json="{\"customer_id\":\"c1\"}" --integrations-json="[{\"name\":\"mock\", \"config\":{\"k\":\"v\"}}]" --integrations-dir=.\integrations --pinpoint-root=.\next
```