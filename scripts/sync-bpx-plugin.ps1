[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$LyraRoot,

    [switch]$Force
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Test-WindowsOrUncPath {
    param([Parameter(Mandatory = $true)][string]$Path)

    if ($Path -match '^[A-Za-z]:[\\/]') {
        return $true
    }

    if ($Path.StartsWith('\\')) {
        return $true
    }

    return $false
}

function Get-FileMap {
    param([Parameter(Mandatory = $true)][string]$Root)

    $result = @{}
    $rootItem = Get-Item -LiteralPath $Root

    $files = Get-ChildItem -LiteralPath $Root -File -Recurse
    foreach ($file in $files) {
        $relative = $file.FullName.Substring($rootItem.FullName.Length).TrimStart([char[]]@('\', '/'))
        $result[$relative] = $file.FullName
    }

    return $result
}

function Test-DirectoryEquivalent {
    param(
        [Parameter(Mandatory = $true)][string]$Source,
        [Parameter(Mandatory = $true)][string]$Destination
    )

    $sourceMap = Get-FileMap -Root $Source
    $destMap = Get-FileMap -Root $Destination

    if ($sourceMap.Count -ne $destMap.Count) {
        return $false
    }

    foreach ($relative in $sourceMap.Keys) {
        if (-not $destMap.ContainsKey($relative)) {
            return $false
        }

        $srcHash = (Get-FileHash -LiteralPath $sourceMap[$relative] -Algorithm SHA256).Hash
        $dstHash = (Get-FileHash -LiteralPath $destMap[$relative] -Algorithm SHA256).Hash
        if ($srcHash -ne $dstHash) {
            return $false
        }
    }

    return $true
}

if (-not (Test-WindowsOrUncPath -Path $LyraRoot)) {
    throw "LyraRoot must be a Windows drive path or UNC path. Input: $LyraRoot"
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = (Resolve-Path (Join-Path $scriptDir '..')).Path
$sourcePluginDir = Join-Path $repoRoot 'testdata/BPXFixtureGenerator'

if (-not (Test-Path -LiteralPath $sourcePluginDir -PathType Container)) {
    throw "Plugin source directory not found: $sourcePluginDir"
}

if (-not (Test-Path -LiteralPath $LyraRoot -PathType Container)) {
    throw "LyraRoot not found: $LyraRoot"
}

$lyraPluginsDir = Join-Path $LyraRoot 'Plugins'
if (-not (Test-Path -LiteralPath $lyraPluginsDir -PathType Container)) {
    throw "Lyra plugins directory not found: $lyraPluginsDir"
}

$destinationPluginDir = Join-Path $lyraPluginsDir 'BPXFixtureGenerator'

if (Test-Path -LiteralPath $destinationPluginDir -PathType Container) {
    if (-not $Force) {
        $same = Test-DirectoryEquivalent -Source $sourcePluginDir -Destination $destinationPluginDir
        if (-not $same) {
            throw "Destination already exists and differs from source. Re-run with -Force to overwrite: $destinationPluginDir"
        }

        Write-Host "BPX plugin already synced: $destinationPluginDir"
    }
    else {
        Remove-Item -LiteralPath $destinationPluginDir -Recurse -Force
        Copy-Item -LiteralPath $sourcePluginDir -Destination $destinationPluginDir -Recurse
        Write-Host "BPX plugin synced (force overwrite): $destinationPluginDir"
    }
}
else {
    Copy-Item -LiteralPath $sourcePluginDir -Destination $destinationPluginDir -Recurse
    Write-Host "BPX plugin synced: $destinationPluginDir"
}

$upluginPath = Join-Path $destinationPluginDir 'BPXFixtureGenerator.uplugin'
if (-not (Test-Path -LiteralPath $upluginPath -PathType Leaf)) {
    throw "Sync validation failed: missing .uplugin file at $upluginPath"
}

$buildFiles = @(Get-ChildItem -LiteralPath $destinationPluginDir -File -Recurse -Filter '*.Build.cs')
if ($buildFiles.Count -eq 0) {
    throw "Sync validation failed: no .Build.cs file found under $destinationPluginDir"
}

Write-Host "Sync validation passed: plugin descriptor and build files are present."
