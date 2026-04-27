param(
  [Parameter(Mandatory = $true)]
  [ValidateSet("resolve-metadata", "rewrite-version-manifests", "verify-version-manifests", "build-sidecar")]
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

function New-SemVerInfo {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [Parameter(Mandatory = $true)]
    [System.Text.RegularExpressions.Match]$Match,
    [Parameter(Mandatory = $true)]
    [string]$TagName
  )

  $prereleaseIdentifiers = @()
  if ($Match.Groups["prerelease"].Success) {
    $prereleaseIdentifiers = @(
      $Match.Groups["prerelease"].Value.Split(".") | ForEach-Object {
        $isNumeric = $_ -match "^\d+$"
        [pscustomobject]@{
          raw = $_
          is_numeric = $isNumeric
          numeric_value = if ($isNumeric) { [int64]$_ } else { $null }
        }
      }
    )
  }

  [pscustomobject]@{
    version = $Version
    tag_name = $TagName
    major = [int64]$Match.Groups["major"].Value
    minor = [int64]$Match.Groups["minor"].Value
    patch = [int64]$Match.Groups["patch"].Value
    prerelease = $prereleaseIdentifiers
  }
}

function Compare-SemVer {
  param(
    [Parameter(Mandatory = $true)]
    [pscustomobject]$Left,
    [Parameter(Mandatory = $true)]
    [pscustomobject]$Right
  )

  foreach ($part in "major", "minor", "patch") {
    if ($Left.$part -gt $Right.$part) { return 1 }
    if ($Left.$part -lt $Right.$part) { return -1 }
  }

  $leftHasPrerelease = $Left.prerelease.Count -gt 0
  $rightHasPrerelease = $Right.prerelease.Count -gt 0
  if (-not $leftHasPrerelease -and -not $rightHasPrerelease) { return 0 }
  if (-not $leftHasPrerelease) { return 1 }
  if (-not $rightHasPrerelease) { return -1 }

  $identifierCount = [Math]::Max($Left.prerelease.Count, $Right.prerelease.Count)
  for ($index = 0; $index -lt $identifierCount; $index++) {
    if ($index -ge $Left.prerelease.Count) { return -1 }
    if ($index -ge $Right.prerelease.Count) { return 1 }

    $leftIdentifier = $Left.prerelease[$index]
    $rightIdentifier = $Right.prerelease[$index]

    if ($leftIdentifier.is_numeric -and $rightIdentifier.is_numeric) {
      if ($leftIdentifier.numeric_value -gt $rightIdentifier.numeric_value) { return 1 }
      if ($leftIdentifier.numeric_value -lt $rightIdentifier.numeric_value) { return -1 }
      continue
    }

    if ($leftIdentifier.is_numeric -and -not $rightIdentifier.is_numeric) { return -1 }
    if (-not $leftIdentifier.is_numeric -and $rightIdentifier.is_numeric) { return 1 }

    $stringComparison = [string]::CompareOrdinal($leftIdentifier.raw, $rightIdentifier.raw)
    if ($stringComparison -gt 0) { return 1 }
    if ($stringComparison -lt 0) { return -1 }
  }

  return 0
}

function Get-HighestPublishedStableReleaseVersion {
  param(
    [Parameter(Mandatory = $true)]
    [string]$SemVerPattern
  )

  if ([string]::IsNullOrWhiteSpace($env:GITHUB_TOKEN)) {
    throw "GITHUB_TOKEN is required."
  }

  $headers = @{
    Authorization = "Bearer $env:GITHUB_TOKEN"
    Accept = "application/vnd.github+json"
    "X-GitHub-Api-Version" = "2022-11-28"
  }
  $bestVersion = $null

  for ($page = 1; $page -le 10; $page++) {
    $uri = "$env:GITHUB_API_URL/repos/$env:GITHUB_REPOSITORY/releases?per_page=100&page=$page"
    $releases = Invoke-RestMethod -Headers $headers -Uri $uri
    if ($releases.Count -eq 0) { break }

    foreach ($release in $releases) {
      if ($release.draft -or $release.prerelease) { continue }

      $tagName = [string]$release.tag_name
      if (-not $tagName.StartsWith("v")) {
        Write-Warning "Skipping published release tag '$tagName' because it does not start with 'v'."
        continue
      }

      $version = $tagName.Substring(1)
      $match = [regex]::Match($version, $SemVerPattern)
      if (-not $match.Success) {
        Write-Warning "Skipping published release tag '$tagName' because it is not valid SemVer."
        continue
      }

      $candidate = New-SemVerInfo -Version $version -Match $match -TagName $tagName
      if ($null -eq $bestVersion -or (Compare-SemVer -Left $candidate -Right $bestVersion) -gt 0) {
        $bestVersion = $candidate
      }
    }

    if ($releases.Count -lt 100) { break }
  }

  if ($null -eq $bestVersion) {
    throw "Could not determine the highest published stable SemVer release."
  }

  Assert-WindowsInstallerVersionBounds -Version $bestVersion.version -Major $bestVersion.major -Minor $bestVersion.minor -Patch $bestVersion.patch
  return $bestVersion
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
      Write-WorkflowOutput -Name "release_version" -Value $version
      Write-WorkflowOutput -Name "release_is_prerelease" -Value $isPrerelease.ToString().ToLowerInvariant()
      Write-WorkflowOutput -Name "release_bundles" -Value $bundles
      return
    }

    if ($env:GITHUB_REF -eq "refs/heads/main") {
      $baseVersion = $null
      for ($attempt = 0; $attempt -lt 3; $attempt++) {
        try {
          $baseVersion = Get-HighestPublishedStableReleaseVersion -SemVerPattern $semverPattern
          break
        } catch {
          if ($attempt -eq 2) {
            throw "Failed to determine the highest published stable SemVer release after 3 attempts: $_"
          }
          Start-Sleep -Seconds 2
        }
      }

      if ($null -eq $baseVersion) {
        throw "Could not determine base version from the highest published stable release"
      }

      $nextPatch = $baseVersion.patch + 1
      $releaseVersion = "$($baseVersion.major).$($baseVersion.minor).$nextPatch"
      Assert-WindowsInstallerVersionBounds `
        -Version $releaseVersion `
        -Major $baseVersion.major `
        -Minor $baseVersion.minor `
        -Patch $nextPatch

      $compareUrl = "$env:GITHUB_SERVER_URL/$env:GITHUB_REPOSITORY/commits/$env:GITHUB_SHA"
      if ($env:PREVIOUS_SHA -and $env:PREVIOUS_SHA -ne "0000000000000000000000000000000000000000") {
        $compareUrl = "$env:GITHUB_SERVER_URL/$env:GITHUB_REPOSITORY/compare/$env:PREVIOUS_SHA...$env:GITHUB_SHA"
      }

      Write-WorkflowOutput -Name "release_mode" -Value "main"
      Write-WorkflowOutput -Name "release_version" -Value $releaseVersion
      Write-WorkflowOutput -Name "release_is_prerelease" -Value "false"
      Write-WorkflowOutput -Name "release_bundles" -Value "nsis,msi"
      Write-WorkflowOutput -Name "release_compare_url" -Value $compareUrl
      return
    }

    throw "Unsupported ref"
  }

  "rewrite-version-manifests" {
    if ([string]::IsNullOrWhiteSpace($env:RELEASE_VERSION)) {
      throw "RELEASE_VERSION is required."
    }

    $version = $env:RELEASE_VERSION

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
    if ([string]::IsNullOrWhiteSpace($env:RELEASE_VERSION)) {
      throw "RELEASE_VERSION is required."
    }

    $version = $env:RELEASE_VERSION
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
}
