# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Lorenzo Garrone (noire342)
[CmdletBinding()]
param(
    [string]$HostAddress,
    [ValidateRange(1,65535)][int]$Port = 22,
    [string]$Version = 'latest',
    [switch]$Uninstall,
    [switch]$DownloadOnly,
    [string]$Destination
)
$ErrorActionPreference = 'Stop'
$repo = 'noire342/remarkable-quickshare'

function Get-ReleaseBundle {
    param([string]$TargetDirectory)
    $headers = @{ 'User-Agent' = 'remarkable-quickshare-installer'; Accept = 'application/vnd.github+json' }
    if ($Version -eq 'latest') {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -Headers $headers
        $tag = $release.tag_name
    } else { $tag = $Version }
    if ($tag -notmatch '^v\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?$') { throw 'Invalid release version.' }
    $name = 'remarkable-quickshare-linux-armv7.tar.gz'
    $baseUrl = "https://github.com/$repo/releases/download/$tag"
    $bundle = Join-Path $TargetDirectory $name
    $checksums = Join-Path $TargetDirectory 'SHA256SUMS'
    Write-Host "Downloading $tag..."
    Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl/$name" -OutFile $bundle
    Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl/SHA256SUMS" -OutFile $checksums
    $expected = $null
    foreach ($line in (Get-Content -LiteralPath $checksums)) {
        if ($line -match '^([0-9a-fA-F]{64})\s+\*?(.+)$' -and $Matches[2] -eq $name) { $expected = $Matches[1].ToLowerInvariant() }
    }
    if (-not $expected) { throw 'Release checksum is missing.' }
    $actual = (Get-FileHash -LiteralPath $bundle -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { throw 'Release checksum mismatch. Nothing was installed.' }
    Write-Host 'SHA-256 verified.'
    # All dependencies are fetched by the PC's verified HTTPS client, never by
    # the tablet BusyBox wget. The manifest comes from the verified bundle.
    & $tar.Source -xzf $bundle -C $TargetDirectory 'device/dependencies.env'
    if ($LASTEXITCODE -ne 0) { throw 'Release dependency manifest is missing; use the installer shipped with that release.' }
    $dependency = @{}
    foreach ($line in (Get-Content -LiteralPath (Join-Path $TargetDirectory 'device/dependencies.env'))) {
        if ($line -match '^(XOVI_[A-Z0-9_]+)=(.+)$') { $dependency[$Matches[1]] = $Matches[2] }
    }
    if ($dependency.XOVI_SHA256 -notmatch '^[0-9a-f]{64}$' -or $dependency.XOVI_URL -notmatch '^https://github\.com/asivery/rm-xovi-extensions/releases/download/[A-Za-z0-9.-]+/xovi-arm32\.tar\.gz$') {
        throw 'Invalid XOVI dependency manifest.'
    }
    $dependencyDirectory = Join-Path $TargetDirectory 'dependencies'
    New-Item -ItemType Directory -Force -Path $dependencyDirectory | Out-Null
    $dependencyArchive = Join-Path $dependencyDirectory 'xovi-arm32.tar.gz'
    Write-Host "Downloading the pinned XOVI dependency ($($dependency.XOVI_VERSION))..."
    Invoke-WebRequest -UseBasicParsing -Uri $dependency.XOVI_URL -OutFile $dependencyArchive
    if ((Get-FileHash -LiteralPath $dependencyArchive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $dependency.XOVI_SHA256) {
        throw 'XOVI dependency checksum mismatch. Nothing was installed.'
    }
    Write-Host 'XOVI SHA-256 verified.'
    return $bundle
}

if (-not $DownloadOnly) {
    if (-not $HostAddress) {
        $HostAddress = Read-Host 'reMarkable IP address (Enter for USB: 10.11.99.1)'
        if (-not $HostAddress) { $HostAddress = '10.11.99.1' }
    }
    if ($HostAddress -notmatch '^[A-Za-z0-9][A-Za-z0-9.:-]*$') { throw 'Enter a valid IP address or hostname, without a username or shell characters.' }
    $ssh = Get-Command ssh -ErrorAction SilentlyContinue
    if (-not $ssh) { throw 'OpenSSH Client is required. Enable the Windows OpenSSH Client optional feature, then run again.' }
}
if (-not $Uninstall) {
    $tar = Get-Command tar -ErrorAction SilentlyContinue
    if (-not $tar) { throw 'The Windows tar command is required. It is included in current Windows 10/11 installations.' }
}

# TLS verification stays on. Tablet BusyBox wget is deliberately not used to fetch releases.
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
$temporary = $null
try {
    if ($Uninstall) {
        if ($DownloadOnly) { throw '-Uninstall cannot be combined with -DownloadOnly.' }
        & $ssh.Source -p $Port -o StrictHostKeyChecking=ask "root@$HostAddress" 'sh /home/root/quickshare/uninstall.sh'
        if ($LASTEXITCODE -ne 0) { throw 'Uninstall did not complete; inspect the message above.' }
        return
    }
    if ($DownloadOnly) {
        if (-not $Destination) { $Destination = Join-Path (Get-Location) 'remarkable-quickshare-download' }
        $target = [IO.Path]::GetFullPath($Destination)
        New-Item -ItemType Directory -Force -Path $target | Out-Null
    } else {
        $temporary = Join-Path ([IO.Path]::GetTempPath()) ('remarkable-quickshare-' + [guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path $temporary | Out-Null
        $target = $temporary
    }
    $bundle = Get-ReleaseBundle -TargetDirectory $target
    if ($DownloadOnly) { Write-Host "Verified bundle and dependencies saved to $target"; return }

    # One outer archive carries both separately verified upstream packages.
    # Keeping a single SSH session means the password is requested only once.
    $transportRoot = Join-Path $temporary 'transport'
    New-Item -ItemType Directory -Force -Path (Join-Path $transportRoot 'dependencies') | Out-Null
    Copy-Item -LiteralPath $bundle -Destination (Join-Path $transportRoot 'receiver.tar.gz')
    Copy-Item -LiteralPath (Join-Path $target 'dependencies/xovi-arm32.tar.gz') -Destination (Join-Path $transportRoot 'dependencies/xovi-arm32.tar.gz')
    $transport = Join-Path $temporary 'ssh-install.tar'
    & $tar.Source -cf $transport -C $transportRoot 'receiver.tar.gz' 'dependencies/xovi-arm32.tar.gz'
    if ($LASTEXITCODE -ne 0) { throw 'Could not prepare the SSH installation archive.' }

    Write-Host ''
    Write-Host 'Connecting as root. Confirm the tablet host key and enter its SSH password when requested.'
    Write-Host 'The password is shown on the tablet under Help / Copyrights and licenses.'
    $remote = 'set -eu; umask 077; d=$(mktemp -d /home/root/.quickshare-install.XXXXXX); trap ''rm -rf "$d"'' EXIT HUP INT TERM; tar -xf - -C "$d"; mkdir "$d/payload"; tar -xzf "$d/receiver.tar.gz" -C "$d/payload"; mkdir -p "$d/payload/dependencies"; mv "$d/dependencies/xovi-arm32.tar.gz" "$d/payload/dependencies/"; sh "$d/payload/device/install-device.sh"'
    # Write bytes directly to SSH. PowerShell 5.1 text pipelines corrupt binary tar streams.
    $start = New-Object Diagnostics.ProcessStartInfo
    $start.FileName = $ssh.Source
    $escapedCommand = $remote.Replace('"','\"')
    $start.Arguments = "-p $Port -o StrictHostKeyChecking=ask root@$HostAddress `"$escapedCommand`""
    $start.UseShellExecute = $false
    $start.RedirectStandardInput = $true
    $process = New-Object Diagnostics.Process
    $process.StartInfo = $start
    if (-not $process.Start()) { throw 'Could not start SSH.' }
    $inputFile = [IO.File]::OpenRead($transport)
    try {
        $inputFile.CopyTo($process.StandardInput.BaseStream)
        $process.StandardInput.Close()
        $process.WaitForExit()
        if ($process.ExitCode -ne 0) { throw "SSH installer failed (exit $($process.ExitCode))." }
    } finally {
        $inputFile.Dispose()
        if (-not $process.HasExited) { $process.Kill(); $process.WaitForExit() }
        $process.Dispose()
    }
} finally {
    if ($temporary) {
        $resolved = [IO.Path]::GetFullPath($temporary)
        $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
        if ($resolved.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -and [IO.Path]::GetFileName($resolved).StartsWith('remarkable-quickshare-')) {
            Remove-Item -LiteralPath $resolved -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}
