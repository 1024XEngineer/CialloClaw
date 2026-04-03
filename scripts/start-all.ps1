Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$root = Resolve-Path "$PSScriptRoot\.."
$coreScript = Join-Path $root 'scripts\start-core.ps1'
$shellScript = Join-Path $root 'scripts\start-shell.ps1'

Start-Process powershell -ArgumentList @('-NoExit', '-ExecutionPolicy', 'Bypass', '-File', $coreScript) | Out-Null
Start-Sleep -Seconds 2
Start-Process powershell -ArgumentList @('-NoExit', '-ExecutionPolicy', 'Bypass', '-File', $shellScript) | Out-Null
