## Mock integration

### Export command

Windows Powershell
```
.\agent-next.exe export --% --agent-config-json="{\"customer_id\":\"c1\"}" --integrations-json="[{\"name\":\"mock\", \"config\":{\"k\":\"v\"}}]" --integrations-dir=.\integrations --pinpoint-root=.\next
```

```
# Export
go run . export --agent-config-json='{"customer_id":"c1"}' --integrations-json='[{"name":"mock", "config":{"k":"v"}}]'
# Onboarding users
go run . export-onboard-data --agent-config-json='{"customer_id":"c1"}' --integrations-json='[{"name":"mock", "config":{"k":"v"}}]' --object-type=users
# Onboarding projects
go run . export-onboard-data --agent-config-json='{"customer_id":"c1"}' --integrations-json='[{"name":"mock", "config":{"k":"v"}}]' --object-type=projects
```