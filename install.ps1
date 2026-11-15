# Installs the osf CLI (Open Story Format) on Windows.
#
#   irm https://raw.githubusercontent.com/lostwood-games/open-story-foundation/main/install.ps1 | iex
#
# Environment:
#   OSF_VERSION      version to install, e.g. 0.1.0 (default: the latest release)
#   OSF_INSTALL_DIR  where to put osf.exe (default: %LOCALAPPDATA%\Programs\osf)
$ErrorActionPreference = 'Stop'

$repo = 'https://github.com/lostwood-games/open-story-foundation'
$dir = if ($env:OSF_INSTALL_DIR) { $env:OSF_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\osf' }
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$asset = "osf_windows_$arch.zip"
$base = "$repo/releases/latest/download"
if ($env:OSF_VERSION) { $base = "$repo/releases/download/v$($env:OSF_VERSION.TrimStart('v'))" }
if ($env:OSF_DOWNLOAD_URL) { $base = $env:OSF_DOWNLOAD_URL } # a folder holding the assets, for testing

$tmp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  Write-Host "Downloading $base/$asset"
  Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile (Join-Path $tmp $asset)
  Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')

  $line = Select-String -Path (Join-Path $tmp 'checksums.txt') -Pattern " $([regex]::Escape($asset))$" | Select-Object -First 1
  $want = if ($line) { $line.Line.Split(' ')[0] } else { '' }
  $got = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $asset)).Hash.ToLower()
  if (-not $want -or $want -ne $got) { throw "osf: checksum mismatch for $asset" }

  Expand-Archive -Path (Join-Path $tmp $asset) -DestinationPath (Join-Path $tmp 'x') -Force
  New-Item -ItemType Directory -Force -Path $dir | Out-Null
  Copy-Item (Join-Path $tmp 'x\osf.exe') (Join-Path $dir 'osf.exe') -Force
} finally {
  Remove-Item -Recurse -Force $tmp
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $dir) {
  [Environment]::SetEnvironmentVariable('Path', "$userPath;$dir", 'User')
  $env:Path = "$env:Path;$dir"
  Write-Host "Added $dir to your PATH. Open a new terminal to use osf."
}
& (Join-Path $dir 'osf.exe') version
