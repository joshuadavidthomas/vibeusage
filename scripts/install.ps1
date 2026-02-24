$ErrorActionPreference = "Stop"

$Repo = "joshuadavidthomas/vibeusage"
$InstallDir = if ($env:VIBEUSAGE_INSTALL_DIR) { $env:VIBEUSAGE_INSTALL_DIR } else { Join-Path $HOME ".local\bin" }
$Version = if ($env:VIBEUSAGE_VERSION) { $env:VIBEUSAGE_VERSION } else { "latest" }

$archRaw = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
$arch = switch ($archRaw) {
  "x64" { "amd64" }
  "arm64" { "arm64" }
  default { throw "Unsupported architecture: $archRaw" }
}

$asset = "vibeusage_windows_${arch}.zip"
$checksums = "checksums.txt"

if ($Version -eq "latest") {
  $downloadUrl = "https://github.com/$Repo/releases/latest/download/$asset"
  $checksumsUrl = "https://github.com/$Repo/releases/latest/download/$checksums"
} else {
  $tag = if ($Version.StartsWith("v")) { $Version } else { "v$Version" }
  $downloadUrl = "https://github.com/$Repo/releases/download/$tag/$asset"
  $checksumsUrl = "https://github.com/$Repo/releases/download/$tag/$checksums"
}

$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("vibeusage-install-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmpDir | Out-Null

$zipPath = Join-Path $tmpDir $asset
$checksumsPath = Join-Path $tmpDir $checksums

try {
  Write-Host "Downloading $asset..."
  Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath
  Invoke-WebRequest -Uri $checksumsUrl -OutFile $checksumsPath

  $expected = $null
  foreach ($line in Get-Content $checksumsPath) {
    if ([string]::IsNullOrWhiteSpace($line)) { continue }
    $parts = $line -split "\s+"
    if ($parts.Count -lt 2) { continue }
    $name = $parts[1].TrimStart("*")
    if ($name -eq $asset) {
      $expected = $parts[0].ToLowerInvariant()
      break
    }
  }

  if (-not $expected) {
    throw "Could not find checksum entry for $asset"
  }

  $actual = (Get-FileHash -Algorithm SHA256 -Path $zipPath).Hash.ToLowerInvariant()
  if ($expected -ne $actual) {
    throw "Checksum mismatch for $asset (expected $expected, got $actual)"
  }

  Expand-Archive -Path $zipPath -DestinationPath $tmpDir -Force
  $binaryPath = Join-Path $tmpDir "vibeusage.exe"
  if (-not (Test-Path $binaryPath)) {
    throw "Release archive did not contain vibeusage.exe"
  }

  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  Copy-Item -Path $binaryPath -Destination (Join-Path $InstallDir "vibeusage.exe") -Force

  Write-Host "Installed vibeusage to $(Join-Path $InstallDir 'vibeusage.exe')"
  Write-Host "Run: vibeusage --version"
}
finally {
  Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
