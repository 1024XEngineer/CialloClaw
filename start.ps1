param(
    [string]$BackendAddr = $(if ($env:CIALLO_CLAW_ADDR) { $env:CIALLO_CLAW_ADDR } else { '127.0.0.1:17888' }),
    [string]$QtPrefixPath = $env:CIALLO_CLAW_QT_PREFIX
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$goExe = 'E:\go\go1.24.13.windows-amd64\go\bin\go.exe'
$backendDir = Join-Path $repoRoot 'backend'
$frontendDir = Join-Path $repoRoot 'frontend'
$backendBuildDir = Join-Path $backendDir 'build'
$frontendBuildDir = Join-Path $frontendDir 'build'
$backendExe = Join-Path $backendBuildDir 'cialloclaw_backend.exe'

function Resolve-QtPrefix {
    param([string]$ExplicitPath)

    $candidates = New-Object System.Collections.Generic.List[string]
    if ($ExplicitPath) { $candidates.Add($ExplicitPath) }
    if ($env:CMAKE_PREFIX_PATH) {
        foreach ($item in ($env:CMAKE_PREFIX_PATH -split ';')) {
            if ($item) { $candidates.Add($item) }
        }
    }

    foreach ($candidate in $candidates) {
        $candidate = $candidate.Trim('"')
        if ($candidate -and (Test-Path (Join-Path $candidate 'lib\cmake\Qt6'))) {
            return $candidate
        }
    }

    foreach ($root in @('E:\Qt', 'C:\Qt', 'D:\Qt')) {
        if (-not (Test-Path $root)) { continue }
        foreach ($versionDir in (Get-ChildItem -Path $root -Directory | Sort-Object Name -Descending)) {
            foreach ($kitDir in (Get-ChildItem -Path $versionDir.FullName -Directory | Sort-Object Name)) {
                if (Test-Path (Join-Path $kitDir.FullName 'lib\cmake\Qt6')) {
                    return $kitDir.FullName
                }
            }
        }
    }

    throw 'Qt6 prefix not found. Set CIALLO_CLAW_QT_PREFIX first.'
}

function Find-FrontendExe {
    param([string]$BuildDir)

    $exe = Get-ChildItem -Path $BuildDir -Filter 'cialloclaw_frontend.exe' -Recurse -File -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1

    if (-not $exe) {
        throw 'Frontend executable not found. Check the Qt build output.'
    }

    return $exe.FullName
}

function Wait-BackendReady {
    param([string]$BaseUrl)

    $deadline = (Get-Date).AddSeconds(20)
    $probeUrl = "$BaseUrl/api/bootstrap"

    while ((Get-Date) -lt $deadline) {
        try {
            $null = Invoke-RestMethod -Uri $probeUrl -Method Get -TimeoutSec 2
            return
        }
        catch {
            Start-Sleep -Milliseconds 500
        }
    }

    throw "Backend did not become ready at $probeUrl"
}

if (-not (Test-Path $goExe)) {
    throw "Go not found: $goExe"
}

if (-not (Get-Command cmake -ErrorAction SilentlyContinue)) {
    throw 'cmake not found. Install it and add it to PATH.'
}

$qtPrefix = Resolve-QtPrefix $QtPrefixPath
$qtBin = Join-Path $qtPrefix 'bin'
$qtPlugins = Join-Path $qtPrefix 'plugins'
$qtQml = Join-Path $qtPrefix 'qml'

$env:CIALLO_CLAW_ADDR = $BackendAddr
$env:CIALLO_CLAW_BACKEND_URL = "http://$BackendAddr"
$env:PATH = "$qtBin;$env:PATH"
$env:QT_PLUGIN_PATH = $qtPlugins
$env:QT_QPA_PLATFORM_PLUGIN_PATH = (Join-Path $qtPlugins 'platforms')
$env:QML_IMPORT_PATH = $qtQml
$env:QML2_IMPORT_PATH = $qtQml

New-Item -ItemType Directory -Force $backendBuildDir | Out-Null
New-Item -ItemType Directory -Force $frontendBuildDir | Out-Null

Write-Host 'Building backend...'
Push-Location $backendDir
try {
    & $goExe build -o $backendExe .
    if ($LASTEXITCODE -ne 0) { throw 'Backend build failed.' }
}
finally {
    Pop-Location
}

Write-Host 'Configuring/building frontend...'
$cmakeArgs = @('-S', $frontendDir, '-B', $frontendBuildDir, "-DCMAKE_PREFIX_PATH=$qtPrefix", '-DCMAKE_BUILD_TYPE=Release')
if (Get-Command ninja -ErrorAction SilentlyContinue) {
    $cmakeArgs = @('-G', 'Ninja') + $cmakeArgs
}

& cmake @cmakeArgs
if ($LASTEXITCODE -ne 0) { throw 'Frontend CMake configure failed.' }

& cmake --build $frontendBuildDir --config Release
if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed.' }

$frontendExe = Find-FrontendExe $frontendBuildDir

$backendProc = $null
$frontendProc = $null

try {
    Write-Host 'Starting backend...'
    $backendProc = Start-Process -FilePath $backendExe -WorkingDirectory $backendBuildDir -PassThru

    Wait-BackendReady "http://$BackendAddr"

    Write-Host 'Starting frontend...'
    $frontendProc = Start-Process -FilePath $frontendExe -WorkingDirectory (Split-Path -Parent $frontendExe) -PassThru

    Wait-Process -Id $frontendProc.Id
}
finally {
    if ($frontendProc -and -not $frontendProc.HasExited) {
        Stop-Process -Id $frontendProc.Id -Force -ErrorAction SilentlyContinue
    }
    if ($backendProc -and -not $backendProc.HasExited) {
        Stop-Process -Id $backendProc.Id -Force -ErrorAction SilentlyContinue
    }
}
