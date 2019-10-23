## Dev workflow

When developing integrations you do not need to use the backend. It is more convenient to call export and similar commands directly. See next section for an example.

### Running integrations without backend

```
Windows powershell
Export
.\agent-next.exe export 2>&1 > logs.txt --% --agent-config-json="{\"customer_id\":\"c1\"}" --integrations-json="[{\"name\":\"mock\", \"config\":{\"k\":\"v\"}}]" --pinpoint-root=.

Onboarding data
.\agent-next.exe export-onboard-data 2>&1 > logs.txt --% --agent-config-json="{\"customer_id\":\"c1\",\"skip_git\":true}" --integrations-json="[{\"name\":\"jira-hosted\", \"config\":{\"username\":\"XXX\", \"password\":\"XXX\", \"url\":\"https://xxxxxxxxxxxxxx\"}}]" --pinpoint-root=. --object-type=projects

Getting logs
Get-Content .\logs.txt -Wait -Tail 10
```

### Checking exported data
When checking exported data is it often needed to look for a specific id or some fields. Using zcat with jq is often sufficient.

One problem is that we generate multiple file per each type, and zcat * does not work on MacOS.

The following workaround works for fish shell:

```
# add this function to ~/.config/fish/config.fish

# similar to zcat * | jq .
# this is required because on MacOS you need to use zcat < file, otherwise zcat wants .Z suffix attached
# Usage:
# in ./sourcecode.Branch
# zcatall | less
function zcatall
	if test -e ./zcatall
		cat ./zcatall
	else
		for f in *.json.gz; zcat < $f | jq . >> ./zcatall; end; cat ./zcatall
	end
end
```