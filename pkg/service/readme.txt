Location of details os log files. We do not normally need them, since we log to pinpoint specific log file anyway.

# Windows
- Open Event Viewer
- Windows Logs -> Application

# MacOS
cat ~/Library/LaunchAgents/SERVICE_NAME.plist
see StandardOutPath
cat /usr/local/var/log/SERVICE_NAME.out.log

# Alpine (openrc)
- /var/log/SERVICE_NAME.log 
- /var/log/SERVICE_NAME.err

# Ubuntu Bionic(docker) (service)
- /var/log/SERVICE_NAME.log 
- /var/log/SERVICE_NAME.err