## 
## This is the installer for the Pinpoint Agent.
##
## For more information, see: https://github.com/pinpt/agent

Write-Host "
    ____  _                   _       __ 
   / __ \(_)___  ____  ____  (_)___  / /_
  / /_/ / / __ \/ __ \/ __ \/ / __ \/ __/
 / ____/ / / / / /_/ / /_/ / / / / / /_  
/_/   /_/_/ /_/ .___/\____/_/_/ /_/\__/  
             /_/ 
"

$agent_file_name = "pinpoint-agent.exe"
$release_url = "https://install.pinpt.io/$agent_file_name"
$install_dir = "C:\pinpoint-agent"
$install_file = "$install_dir\$agent_file_name"

Write-Host "Dowloading latest release from $release_url"
New-Item -ItemType Directory -Force -Path $install_dir | Out-Null
Invoke-WebRequest $release_url -Out $install_file

Write-Host 'Adding install dir to PATH'
$env:PATH = "${install_dir};" + $env:PATH
[Environment]::SetEnvironmentVariable('PATH', $env:PATH, [EnvironmentVariableTarget]::Machine)

Write-Host 'Running pinpoint-agent to check if it was properly installed. Installed version:'
pinpoint-agent version

Write-Host 'Installation successful!'