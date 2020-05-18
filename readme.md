<div align="center">
	<img width="500" src="_docs/logo.svg" alt="pinpt-logo">
</div>

<p align="center" color="#6a737d">
	<strong>Agent is the software that collects and deliver performance details to the Pinpoint Cloud</strong>
</p>

- [Build from source](./_docs/build.md)
- [Architecture](./_docs/architecture.md)
- [Hidden features](./_docs/hidden_features.md)
- [Development workflow](./_docs/dev_workflow.md)
- [Exported data](./_docs/exported_data.md)
- [Managing agent service](./_docs/managing_agent_service.md)
- [Release](./_docs/release.md)
- [Troubleshooting production issues (Pinpoint only)](https://www.notion.so/Export-Investigation-8b72b268328542d8a78ff21967117fe2)
- [Mutations](./_docs/mutations.md)

## Install

#### Windows

To install latest release, run the following in powershell.

```
Set-ExecutionPolicy Bypass -Scope Process -Force
iex ((New-Object System.Net.WebClient).DownloadString('https://install.pinpt.io/install.ps1'))
```

#### Linux

To install latest release, run the following in your shell.

```
bash -c "$(curl -sSL https://install.pinpt.io/install.sh)"
```

#### Docker

```
docker pull pinpt/agent
docker run -it --rm --name pinpoint_agent -v `pwd`/pinpoint:/pinpoint pinpt/agent enroll --pinpoint-root /pinpoint <ENROLL_CODE>
```

### Required git version

| Version                             | Notes  
| -------------                       | -------- 
| 2.20.1             | Default macos version. Tested as working.
| 2.13               | Released on 2017-05. Introduced clone --no-tags flag. Should work.
| <2.13              | We do not support versions older than this.

## Integration docs

### Sourcecode
- [Microsoft Azure DevOps and TFS](./integrations/azure/readme.md)
- [Bitbucket](./integrations/bitbucket/readme.md)
- [GitHub](./integrations/github/readme.md)
- [GitLab](./integrations/gitlab/readme.md)

### Issue tracking
- [Microsoft Azure DevOps and TFS](./integrations/azure/readme.md)
- [GitLab](./integrations/gitlab/readme.md)
- [Jira](./integrations/jira/readme.md)

### Calendar
- [Google Calendar](./integrations/gcal/readme.md)
- [Office 365](./integrations/office365/readme.md)

### Other integrations
- [SonarQube](./integrations/sonarqube/readme.md)

## License
All of this code is Copyright © 2018-2020 by Pinpoint Software, Inc. Licensed under the MIT License
