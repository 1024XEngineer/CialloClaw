param(
  [Parameter(Mandatory = $true)]
  [ValidateSet("resolve-metadata", "rewrite-version-manifests", "verify-version-manifests", "build-sidecar", "remove-release-by-tag")]
  [string]$Action
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-WorkflowOutput {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Name,
    [Parameter(Mandatory = $true)]
    [string]$Value
  )

  if ([string]::IsNullOrWhiteSpace($env:GITHUB_OUTPUT)) {
    throw "GITHUB_OUTPUT is not available."
  }

  "$Name=$Value" >> $env:GITHUB_OUTPUT
}

function New-GitHubApiHeaders {
  if ([string]::IsNullOrWhiteSpace($env:GITHUB_TOKEN)) {
    throw "GITHUB_TOKEN is required."
  }

  return @{
    Authorization = "Bearer $env:GITHUB_TOKEN"
    Accept = "application/vnd.github+json"
    "X-GitHub-Api-Version" = "2022-11-28"
  }
}

function Invoke-GitHubApiRequest {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Method,
    [Parameter(Mandatory = $true)]
    [string]$Uri
  )

  $headers = New-GitHubApiHeaders
  try {
    return Invoke-RestMethod -Headers $headers -Method $Method -Uri $Uri
  } catch {
    $statusCode = $_.Exception.Response.StatusCode.value__
    if ($statusCode -eq 404) {
      return $null
    }

    throw
  }
}

function Assert-WindowsInstallerVersionBounds {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [Parameter(Mandatory = $true)]
    [int64]$Major,
    [Parameter(Mandatory = $true)]
    [int64]$Minor,
    [Parameter(Mandatory = $true)]
    [int64]$Patch
  )

  if ($Major -gt 255 -or $Minor -gt 255 -or $Patch -gt 65535) {
    throw "Version $Version exceeds MSI limits (major<=255, minor<=255, patch<=65535)."
  }
}

function Remove-GitHubReleaseByTag {
  param(
    [Parameter(Mandatory = $true)]
    [string]$TagName
  )

  $releaseUri = "$env:GITHUB_API_URL/repos/$env:GITHUB_REPOSITORY/releases/tags/$TagName"
  $release = Invoke-GitHubApiRequest -Method "GET" -Uri $releaseUri
  if ($null -eq $release) {
    return
  }

  Invoke-GitHubApiRequest -Method "DELETE" -Uri "$env:GITHUB_API_URL/repos/$env:GITHUB_REPOSITORY/releases/$($release.id)" | Out-Null
}

function Remove-GitHubTagRef {
  param(
    [Parameter(Mandatory = $true)]
    [string]$TagName
  )

  $refUri = "$env:GITHUB_API_URL/repos/$env:GITHUB_REPOSITORY/git/refs/tags/$TagName"
  $tagRef = Invoke-GitHubApiRequest -Method "GET" -Uri $refUri
  if ($null -eq $tagRef) {
    return
  }

  Invoke-GitHubApiRequest -Method "DELETE" -Uri $refUri | Out-Null
}

function Get-PackageVersionFromCargoToml {
  param(
    [Parameter(Mandatory = $true)]
    [string]$CargoPath
  )

  $insidePackage = $false
  foreach ($line in Get-Content $CargoPath) {
    if ($line -match "^\s*\[package\]\s*$") {
      $insidePackage = $true
      continue
    }

    if ($insidePackage -and $line -match "^\s*\[.*\]\s*$") {
      break
    }

    if ($insidePackage -and $line -match '^\s*version\s*=\s*"([^"]+)"\s*$') {
      return $Matches[1]
    }
  }

  return $null
}

switch ($Action) {
  "resolve-metadata" {
    $semverPattern = '^(?<major>0|[1-9]\d*)\.(?<minor>0|[1-9]\d*)\.(?<patch>0|[1-9]\d*)(?:-(?<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?<build>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$'

    if ($env:GITHUB_REF_TYPE -eq "tag") {
      $version = $env:GITHUB_REF_NAME.Substring(1)
      $match = [regex]::Match($version, $semverPattern)
      if (-not $match.Success) {
        throw "Invalid SemVer tag: $env:GITHUB_REF_NAME"
      }

      Assert-WindowsInstallerVersionBounds `
        -Version $version `
        -Major ([int64]$match.Groups["major"].Value) `
        -Minor ([int64]$match.Groups["minor"].Value) `
        -Patch ([int64]$match.Groups["patch"].Value)

      $isPrerelease = $match.Groups["prerelease"].Success
      $bundles = if ($isPrerelease) { "nsis" } else { "nsis,msi" }

      Write-WorkflowOutput -Name "release_mode" -Value "tag"
      Write-WorkflowOutput -Name "manifest_version" -Value $version
      Write-WorkflowOutput -Name "release_is_prerelease" -Value $isPrerelease.ToString().ToLowerInvariant()
      Write-WorkflowOutput -Name "release_bundles" -Value $bundles
      return
    }

    if ($env:GITHUB_REF -eq "refs/heads/main") {
      Write-WorkflowOutput -Name "release_mode" -Value "tip"
      Write-WorkflowOutput -Name "release_is_prerelease" -Value "true"
      Write-WorkflowOutput -Name "release_bundles" -Value "nsis"
      return
    }

    throw "Unsupported ref"
  }

  "rewrite-version-manifests" {
    if ([string]::IsNullOrWhiteSpace($env:MANIFEST_VERSION)) {
      throw "MANIFEST_VERSION is required."
    }

    $version = $env:MANIFEST_VERSION

    $packageJsonPath = "apps/desktop/package.json"
    $packageJson = Get-Content $packageJsonPath -Raw | ConvertFrom-Json
    $packageJson.version = $version
    $packageJson | ConvertTo-Json -Depth 10 | Set-Content $packageJsonPath -NoNewline

    $tauriConfigPath = "apps/desktop/src-tauri/tauri.conf.json"
    $tauriConfig = Get-Content $tauriConfigPath -Raw | ConvertFrom-Json
    $tauriConfig.version = $version
    $tauriConfig | ConvertTo-Json -Depth 10 | Set-Content $tauriConfigPath -NoNewline

    $cargoPath = "apps/desktop/src-tauri/Cargo.toml"
    $cargoContent = Get-Content $cargoPath -Raw
    $newline = if ($cargoContent.Contains("`r`n")) { "`r`n" } else { "`n" }
    $lines = $cargoContent -split "`r?`n"
    $insidePackage = $false
    $packageVersionFound = $false
    $newLines = @()

    foreach ($line in $lines) {
      if ($line -match "^\s*\[package\]\s*$") {
        $insidePackage = $true
        $newLines += $line
        continue
      }

      if ($insidePackage -and $line -match "^\s*\[.*\]\s*$") {
        $insidePackage = $false
      }

      if ($insidePackage -and $line -match '^\s*version\s*=\s*".*"\s*$') {
        $packageVersionFound = $true
        $line = $line -replace '(?<=version\s*=\s*")[^"]*(?=")', $version
      }

      $newLines += $line
    }

    if (-not $packageVersionFound) {
      throw "Failed to update version in Cargo.toml: [package] version line not found"
    }

    [System.IO.File]::WriteAllText($cargoPath, ($newLines -join $newline))
    return
  }

  "verify-version-manifests" {
    if ([string]::IsNullOrWhiteSpace($env:MANIFEST_VERSION)) {
      throw "MANIFEST_VERSION is required."
    }

    $version = $env:MANIFEST_VERSION
    if ((Get-Content "apps/desktop/package.json" -Raw | ConvertFrom-Json).version -ne $version) {
      throw "package.json mismatch"
    }

    if ((Get-Content "apps/desktop/src-tauri/tauri.conf.json" -Raw | ConvertFrom-Json).version -ne $version) {
      throw "tauri.conf.json mismatch"
    }

    $packageVersion = Get-PackageVersionFromCargoToml -CargoPath "apps/desktop/src-tauri/Cargo.toml"
    if ($packageVersion -ne $version) {
      throw "Cargo.toml [package] version mismatch"
    }

    return
  }

  "build-sidecar" {
    $triple = (rustc -vV | Select-String "^host:").Line.Substring(6).Trim()
    $extension = if ($env:OS -eq "Windows_NT") { ".exe" } else { "" }
    $name = "local-service-$triple$extension"

    New-Item -ItemType Directory -Force "apps/desktop/src-tauri/binaries" | Out-Null
    go build -trimpath -o "apps/desktop/src-tauri/binaries/$name" "./services/local-service/cmd/server"
    Write-WorkflowOutput -Name "sidecar_file_name" -Value $name
    return
  }

  "remove-release-by-tag" {
    if ([string]::IsNullOrWhiteSpace($env:RELEASE_TAG_NAME)) {
      throw "RELEASE_TAG_NAME is required."
    }

    Remove-GitHubReleaseByTag -TagName $env:RELEASE_TAG_NAME
    Remove-GitHubTagRef -TagName $env:RELEASE_TAG_NAME
    return
  }
}
