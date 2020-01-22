## Managing agent service

### Different run-type options

To setup/start the agent you would use the enroll command. It supports multiple run-type options.

### --run-type=service

When service option is selected, the agent will be installed as a standard OS service.

The following platforms are supported for OS service:

- Windows
- Linux: openrc, tested in Alpine
- Linux: systemd, tested in Ubuntu

When installed as OS service, agent service will be automatically started on computer restarts and the logs will be available in a standard way which differs by service manager.

### --run-type=direct

Direct run type option does not install OS service and instead starts the agent directly. This is useful if you want to manage the service differently, or when running in docker.

### --run-type=enroll-only

This is useful to separate enroll and run, mainly useful for development.

### Logging

Irrespective of run-type we maintain logs for the last command run in PINPOINT_ROOT/logs directory. It also has separate log files for different subcomponents, but to see all logs together you would normally use the logs from "enroll" files. Note that these log files are overridden when you restart specific command, so only useful for the last run.

We also ship the logs from export sub-command to the server. Which are then available in admin.

#### OS specific service management

When using run-type=service service management differs by platform.

#### Windows

When running on Windows, service logs only contain information about service start, stop and similar events.

- Open Event Viewer
- Windows Logs -> Application

#### Linux Systemd (Ubuntu)

```
Check status
sudo systemctl status pinpoint-agent

Check logs
All logs from service are available in journalctl

sudo journalctl -u pinpoint-agent

Start/Stop/Restart
sudo systemctl restart pinpoint-agent
```

#### Linux Alpine (OpenRC)

- TODO: incomplete support, can't stop/restart service

```
List services
rc-update -v show 
Show status
rc-service pinpoint-agent ((start, stop, restart, status)

When using OpenRC, all logs from the service would be available in the following locations:

- /var/log/SERVICE_NAME.err
Logs from the service, as well as service start/stop events are logged in err.

- /var/log/SERVICE_NAME.log 
We do not log anything to regular log file.
```

#### MacOS (only for development)

```
cat ~/Library/LaunchAgents/SERVICE_NAME.plist
see StandardOutPath
cat /usr/local/var/log/SERVICE_NAME.out.log
```