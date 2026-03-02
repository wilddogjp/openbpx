[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$LyraRoot,

    [Parameter(Mandatory = $true)]
    [string]$BpxRepoRoot,

    [string]$Scope = '1,2',

    [string]$Include,

    [switch]$Force,

    [string]$EditorCmdPath,

    [switch]$SkipEditorBuild
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

if (-not (Test-WindowsOrUncPath -Path $LyraRoot)) {
    throw "LyraRoot must be a Windows drive path or UNC path. Input: $LyraRoot"
}

if (-not (Test-WindowsOrUncPath -Path $BpxRepoRoot)) {
    throw "BpxRepoRoot must be a Windows drive path or UNC path. Input: $BpxRepoRoot"
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$syncScript = Join-Path $scriptDir 'sync-bpx-plugin.ps1'
if (-not (Test-Path -LiteralPath $syncScript -PathType Leaf)) {
    throw "Sync script not found: $syncScript"
}

$syncArgs = @{
    LyraRoot = $LyraRoot
}
if ($Force) {
    $syncArgs['Force'] = $true
}

& $syncScript @syncArgs

$uprojectPath = Join-Path $LyraRoot 'Lyra.uproject'
if (-not (Test-Path -LiteralPath $uprojectPath -PathType Leaf)) {
    throw "Lyra project file not found: $uprojectPath"
}

if ([string]::IsNullOrWhiteSpace($EditorCmdPath)) {
    $engineRoot = (Resolve-Path (Join-Path $LyraRoot '..\..\..')).Path
    $candidateCmd = Join-Path $engineRoot 'Engine\Binaries\Win64\UnrealEditor-Cmd.exe'
    $candidateEditor = Join-Path $engineRoot 'Engine\Binaries\Win64\UnrealEditor.exe'

    if (Test-Path -LiteralPath $candidateCmd -PathType Leaf) {
        $EditorCmdPath = $candidateCmd
    }
    elseif (Test-Path -LiteralPath $candidateEditor -PathType Leaf) {
        $EditorCmdPath = $candidateEditor
    }
    else {
        $EditorCmdPath = $candidateCmd
    }
}

if (-not (Test-Path -LiteralPath $EditorCmdPath -PathType Leaf)) {
    throw "UnrealEditor-Cmd executable not found: $EditorCmdPath"
}

$editorBinDir = Split-Path -Parent $EditorCmdPath
$resolvedEngineRoot = (Resolve-Path (Join-Path $editorBinDir '..\..')).Path
$buildBatPath = Join-Path $resolvedEngineRoot 'Build\BatchFiles\Build.bat'

if (-not $SkipEditorBuild) {
    if (-not (Test-Path -LiteralPath $buildBatPath -PathType Leaf)) {
        throw "Build.bat not found: $buildBatPath"
    }

    Write-Host 'Building LyraEditor (non-interactive)...'
    $buildArgs = @(
        'LyraEditor',
        'Win64',
        'Development',
        "-Project=$uprojectPath",
        '-WaitMutex',
        '-NoHotReloadFromIDE'
    )
    & $buildBatPath @buildArgs
    $buildExitCode = $LASTEXITCODE
    if ($buildExitCode -ne 0) {
        throw "Build.bat failed with exit code $buildExitCode"
    }
}

$cmdArgs = @(
    $uprojectPath,
    '-run=BPXGenerateFixtures',
    "-BpxRepoRoot=$BpxRepoRoot",
    "-Scope=$Scope",
    '-ini:Engine:[/Script/Engine.Engine]:AssetManagerClassName=/Script/Engine.AssetManager',
    '-Unattended',
    '-stdout',
    '-AllowStdOutLogVerbosity',
    '-NoSplash',
    '-NoP4'
)

if (-not [string]::IsNullOrWhiteSpace($Include)) {
    $cmdArgs += "-Include=$Include"
}

if ($Force) {
    $cmdArgs += '-Force'
}

Write-Host "Running BPX fixture commandlet..."
Write-Host "  Editor: $EditorCmdPath"
Write-Host "  Project: $uprojectPath"
Write-Host "  Scope: $Scope"
if ($Include) {
    Write-Host "  Include: $Include"
}

& $EditorCmdPath @cmdArgs
$exitCode = $LASTEXITCODE
if ($exitCode -ne 0) {
    throw "Commandlet failed with exit code $exitCode"
}

Write-Host 'BPX fixture commandlet completed successfully.'
