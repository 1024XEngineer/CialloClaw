Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$env:NUGET_PACKAGES = Join-Path $env:USERPROFILE '.nuget\packages'
$env:NUGET_FALLBACK_PACKAGES = ''

Push-Location "$PSScriptRoot\..\wpf-shell\CialloClaw.Shell"
try {
    dotnet run
}
finally {
    Pop-Location
}
