param(
    [string]$Configuration = 'Release'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$root = Resolve-Path "$PSScriptRoot\.."
$dist = Join-Path $root 'dist\CialloClaw'
$coreOutDir = Join-Path $dist 'core'
$coreExe = Join-Path $coreOutDir 'go-core.exe'
$wpfProject = Join-Path $root 'wpf-shell\CialloClaw.Shell\CialloClaw.Shell.csproj'

Write-Host "[1/4] Prepare dist directory: $dist"
if (Test-Path $dist) {
    Remove-Item -Recurse -Force $dist
}
New-Item -ItemType Directory -Force -Path $dist | Out-Null
New-Item -ItemType Directory -Force -Path $coreOutDir | Out-Null

Write-Host "[2/4] Build go-core.exe"
Push-Location (Join-Path $root 'go-core')
try {
    $cacheDir = Join-Path $root '.cache\go-build'
    New-Item -ItemType Directory -Force -Path $cacheDir | Out-Null
    $env:GOCACHE = (Resolve-Path $cacheDir).Path
    go build -o $coreExe .
}
finally {
    Pop-Location
}

Write-Host "[3/4] Publish WPF shell as CialloClaw.exe"
$env:NUGET_PACKAGES = Join-Path $env:USERPROFILE '.nuget\packages'
$env:NUGET_FALLBACK_PACKAGES = ''
dotnet publish $wpfProject `
  -c $Configuration `
  -r win-x64 `
  --self-contained true `
  -p:PublishSingleFile=true `
  -p:IncludeNativeLibrariesForSelfExtract=true `
  -p:PublishTrimmed=false `
  -o $dist

Write-Host "[4/4] Copy docs"
Copy-Item -Force (Join-Path $root 'README.md') (Join-Path $dist 'README.md')

Write-Host ''
Write-Host 'Done. Launch this file:'
Write-Host (Join-Path $dist 'CialloClaw.exe')
