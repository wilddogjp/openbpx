param(
    [string]$Version = "",
    [string]$InstallDir = "$HOME\\.local\\bin",
    [string]$Repo = "wilddogjp/openbpx"
)

$ErrorActionPreference = "Stop"

function Normalize-Version {
    param([string]$Raw)
    if ([string]::IsNullOrWhiteSpace($Raw)) { return "" }
    if ($Raw.StartsWith("v")) { return $Raw }
    return "v$Raw"
}

function Resolve-Arch {
    $raw = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
    switch ($raw) {
        "x64" { return "amd64" }
        "arm64" { return "arm64" }
        default { throw "Unsupported architecture: $raw" }
    }
}

if ([string]::IsNullOrWhiteSpace($Version)) {
    $latest = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    $Version = $latest.tag_name
}
$Version = Normalize-Version $Version
$versionNoV = $Version.TrimStart('v')
$arch = Resolve-Arch
$assetName = "bpx_${versionNoV}_windows_${arch}.zip"
$baseUrl = "https://github.com/$Repo/releases/download/$Version"

$tmpRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("bpx-install-" + [guid]::NewGuid().ToString("N"))
$extractDir = Join-Path $tmpRoot "extract"
$checksumPath = Join-Path $tmpRoot "checksums.txt"
$assetPath = Join-Path $tmpRoot $assetName

New-Item -ItemType Directory -Path $tmpRoot -Force | Out-Null

try {
    Invoke-WebRequest -Uri "$baseUrl/checksums.txt" -OutFile $checksumPath
    Invoke-WebRequest -Uri "$baseUrl/$assetName" -OutFile $assetPath

    $line = Get-Content $checksumPath | Where-Object { $_ -match ("\s\s" + [Regex]::Escape($assetName) + "$") } | Select-Object -First 1
    if (-not $line) {
        throw "Checksum entry not found for $assetName"
    }

    $expected = ($line -split '\s+')[0].ToLowerInvariant()
    $actual = (Get-FileHash -Algorithm SHA256 -Path $assetPath).Hash.ToLowerInvariant()
    if ($expected -ne $actual) {
        throw "Checksum verification failed for $assetName`nexpected: $expected`nactual:   $actual"
    }

    Expand-Archive -Path $assetPath -DestinationPath $extractDir -Force

    $binary = Get-ChildItem -Path $extractDir -Filter "bpx.exe" -File -Recurse | Select-Object -First 1
    if (-not $binary) {
        throw "Failed to find bpx.exe in $assetName"
    }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $target = Join-Path $InstallDir "bpx.exe"
    Copy-Item -Path $binary.FullName -Destination $target -Force

    Write-Host "Installed: $target"
    & $target version

    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (-not ($currentPath -split ';' | Where-Object { $_ -eq $InstallDir })) {
        Write-Host "Add to PATH (User) if needed: $InstallDir"
    }
}
finally {
    Remove-Item -Path $tmpRoot -Recurse -Force -ErrorAction SilentlyContinue
}
