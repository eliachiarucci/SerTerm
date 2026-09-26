# Install script for serterm on Windows.
#
# Usage (PowerShell):
#   irm https://raw.githubusercontent.com/eliachiarucci/serterm/main/install.ps1 | iex
#
# Environment variables:
#   SERTERM_VERSION  - version to install (default: latest release)
#   SERTERM_INSTALL  - install directory (default: the directory of the
#                      existing install, if any; otherwise
#                      %LOCALAPPDATA%\Programs\serterm)
#
# Everything runs inside a function so that, when piped into iex, no
# variables or settings leak into the caller's session. Errors are thrown
# rather than calling exit, which would close the caller's window.

function Install-Serterm {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue'  # Invoke-WebRequest is very slow with the progress bar on PowerShell 5.1
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    $repo = 'eliachiarucci/serterm'

    $arch = Get-Arch
    $version = $env:SERTERM_VERSION
    if (-not $version) {
        $version = (Invoke-RestMethod -UseBasicParsing "https://api.github.com/repos/$repo/releases/latest").tag_name
    }
    $version = $version -replace '^v', ''

    $installDir = $env:SERTERM_INSTALL
    if (-not $installDir) {
        # Updates go to the directory of the existing install.
        $existing = Get-Command serterm -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($existing) {
            $installDir = Split-Path -Parent $existing.Source
        } else {
            $installDir = Join-Path $env:LOCALAPPDATA 'Programs\serterm'
        }
    }
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null

    $archive = "serterm_${version}_windows_${arch}.zip"
    $baseUrl = "https://github.com/$repo/releases/download/v$version"

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ("serterm-" + [Guid]::NewGuid())
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Write-Host "Downloading serterm v$version (windows/$arch)..."
        $zip = Join-Path $tmp $archive
        Invoke-WebRequest -UseBasicParsing "$baseUrl/$archive" -OutFile $zip

        $checksums = (Invoke-WebRequest -UseBasicParsing "$baseUrl/checksums.txt").Content
        if ($checksums -is [byte[]]) { $checksums = [Text.Encoding]::UTF8.GetString($checksums) }
        $line = $checksums -split "`n" | Where-Object { $_ -match "\s$([regex]::Escape($archive))\s*$" } | Select-Object -First 1
        if (-not $line) { throw "No checksum found for $archive." }
        $expected = ($line -split '\s+')[0]
        $actual = (Get-FileHash -Algorithm SHA256 $zip).Hash
        if ($actual -ne $expected) { throw "Checksum mismatch for $archive (expected $expected, got $actual)." }

        Expand-Archive -Path $zip -DestinationPath $tmp -Force

        $target = Join-Path $installDir 'serterm.exe'
        $old = "$target.old"
        # A running exe can't be overwritten but can be renamed, which is what
        # lets `serterm update` replace itself. Clean up the previous leftover.
        Remove-Item $old -Force -ErrorAction SilentlyContinue
        try {
            if (Test-Path $target) { Move-Item $target $old -Force }
            Copy-Item (Join-Path $tmp 'serterm.exe') $target -Force
        } catch {
            if ((Test-Path $old) -and -not (Test-Path $target)) { Move-Item $old $target -ErrorAction SilentlyContinue }
            throw "Cannot write to $installDir. Run PowerShell as administrator, or choose a directory with `$env:SERTERM_INSTALL = '<dir>'."
        }
    } finally {
        Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }

    Write-Host "Installed serterm v$version to $target"

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $onPath = ($userPath -split ';') + ($env:Path -split ';') | Where-Object { $_.TrimEnd('\') -ieq $installDir.TrimEnd('\') }
    if (-not $onPath) {
        $newPath = if ($userPath) { "$userPath;$installDir" } else { $installDir }
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        $env:Path = "$env:Path;$installDir"
        Write-Host "Added $installDir to your PATH. Open a new terminal if serterm isn't found."
    }
}

function Get-Arch {
    # PROCESSOR_ARCHITEW6432 is set when a 32-bit PowerShell runs on a 64-bit OS.
    $a = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
    switch ($a) {
        'AMD64' { return 'amd64' }
        'ARM64' { return 'arm64' }
        default { throw "Unsupported architecture: $a" }
    }
}

Install-Serterm
