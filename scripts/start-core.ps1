Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$cacheDir = Join-Path $PSScriptRoot '..\.cache\go-build'
New-Item -ItemType Directory -Force -Path $cacheDir | Out-Null
$env:GOCACHE = (Resolve-Path $cacheDir).Path

Push-Location "$PSScriptRoot\..\go-core"
try {
    go run .
}
finally {
    Pop-Location
}
