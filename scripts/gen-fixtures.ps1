[CmdletBinding()]
param(
    [string]$LyraRoot,

    [string]$BpxRepoRoot,

    [object]$Scope = '1,2',

    [object]$Include,

    [switch]$Force,

    [string]$EditorCmdPath,

    [switch]$SkipEditorBuild,

    [string]$GoldenRoot,

    [string]$UEEngineRoot,

    [string]$ConfigPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Convert-ToNormalizedCsvString {
    param(
        [Parameter(Mandatory = $false)]
        [AllowNull()]
        [object]$Value
    )

    if ($null -eq $Value) {
        return $null
    }

    if ($Value -is [string]) {
        return $Value
    }

    if ($Value -is [System.Array]) {
        return (($Value | ForEach-Object { [string]$_ }) -join ',')
    }

    return [string]$Value
}

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

function Test-IsWslUncPath {
    param([Parameter(Mandatory = $false)][string]$Path)

    if ([string]::IsNullOrWhiteSpace($Path)) {
        return $false
    }

    return $Path.StartsWith('\\wsl.localhost\') -or $Path.StartsWith('\\wsl$\')
}

function Get-LastExitCodeOrZero {
    $lastExitCodeVar = Get-Variable -Name LASTEXITCODE -ErrorAction SilentlyContinue
    if ($null -eq $lastExitCodeVar) {
        $lastExitCodeVar = Get-Variable -Name LASTEXITCODE -Scope Global -ErrorAction SilentlyContinue
    }
    if ($null -eq $lastExitCodeVar) {
        return 0
    }
    return [int]$lastExitCodeVar.Value
}

function Convert-ToCmdArgument {
    param([Parameter(Mandatory = $true)][string]$Value)

    if ($Value -notmatch '[\s"&|<>^()]') {
        return $Value
    }

    return '"' + ($Value -replace '"', '""') + '"'
}

function Invoke-BatchFile {
    param(
        [Parameter(Mandatory = $true)][string]$BatchPath,
        [Parameter(Mandatory = $false)][string[]]$Arguments = @(),
        [Parameter(Mandatory = $false)][string]$WorkingDirectory
    )

    $commandLine = ((@($BatchPath) + @($Arguments)) | ForEach-Object {
            Convert-ToCmdArgument -Value ([string]$_)
        }) -join ' '
    $cmdExePath = $env:ComSpec
    if ([string]::IsNullOrWhiteSpace($cmdExePath)) {
        $cmdExePath = 'C:\WINDOWS\system32\cmd.exe'
    }

    $startInfo = New-Object System.Diagnostics.ProcessStartInfo
    $startInfo.FileName = $cmdExePath
    $startInfo.Arguments = "/d /c call $commandLine"
    $startInfo.UseShellExecute = $false
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $startInfo.CreateNoWindow = $true
    if (-not [string]::IsNullOrWhiteSpace($WorkingDirectory)) {
        $startInfo.WorkingDirectory = $WorkingDirectory
    }

    foreach ($entry in [System.Environment]::GetEnvironmentVariables().GetEnumerator()) {
        $startInfo.EnvironmentVariables[[string]$entry.Key] = [string]$entry.Value
    }

    $process = New-Object System.Diagnostics.Process
    $process.StartInfo = $startInfo
    $null = $process.Start()
    $stdout = $process.StandardOutput.ReadToEnd()
    $stderr = $process.StandardError.ReadToEnd()
    $process.WaitForExit()

    if (-not [string]::IsNullOrEmpty($stdout)) {
        [Console]::Out.Write($stdout)
    }
    if (-not [string]::IsNullOrEmpty($stderr)) {
        [Console]::Error.Write($stderr)
    }

    return [int]$process.ExitCode
}

function Should-RunProfilesSequentially {
    param(
        [Parameter(Mandatory = $true)][object[]]$Profiles,
        [Parameter(Mandatory = $false)][string]$BpxRepoRoot
    )

    if (Test-IsWslUncPath -Path $BpxRepoRoot) {
        return $true
    }

    foreach ($profile in $Profiles) {
        $goldenRoot = [string]$profile.GoldenRoot
        if (Test-IsWslUncPath -Path $goldenRoot) {
            return $true
        }
    }

    return $false
}

function Get-ConfigValue {
    param(
        [Parameter(Mandatory = $false)]
        [object]$Config,

        [Parameter(Mandatory = $true)]
        [string]$Name
    )

    if ($null -eq $Config) {
        return $null
    }

    $property = $Config.PSObject.Properties[$Name]
    if ($null -eq $property) {
        return $null
    }

    return $property.Value
}

function Read-LocalConfig {
    param(
        [Parameter(Mandatory = $true)]
        [string]$DefaultPath,

        [Parameter(Mandatory = $false)]
        [string]$SpecifiedPath
    )

    $resolvedPath = $SpecifiedPath
    if ([string]::IsNullOrWhiteSpace($resolvedPath)) {
        if (-not (Test-Path -LiteralPath $DefaultPath -PathType Leaf)) {
            return $null
        }
        $resolvedPath = $DefaultPath
    }
    elseif (-not (Test-Path -LiteralPath $resolvedPath -PathType Leaf)) {
        throw "Config file not found: $resolvedPath"
    }

    try {
        $raw = Get-Content -LiteralPath $resolvedPath -Raw
        if ([string]::IsNullOrWhiteSpace($raw)) {
            throw 'config file is empty'
        }
        return ($raw | ConvertFrom-Json)
    }
    catch {
        throw "failed to parse config file as JSON ($resolvedPath): $($_.Exception.Message)"
    }
}

function Resolve-EditorExecutable {
    param(
        [Parameter(Mandatory = $true)][string]$LyraRoot,
        [Parameter(Mandatory = $false)][string]$UEEngineRoot,
        [Parameter(Mandatory = $false)][string]$EditorCmdPath
    )

    if (-not [string]::IsNullOrWhiteSpace($EditorCmdPath)) {
        return $EditorCmdPath
    }

    $engineRoot = $UEEngineRoot
    if ([string]::IsNullOrWhiteSpace($engineRoot)) {
        $engineRoot = (Resolve-Path (Join-Path $LyraRoot '..\..\..')).Path
    }

    $candidateCmd = Join-Path $engineRoot 'Engine\Binaries\Win64\UnrealEditor-Cmd.exe'
    $candidateEditor = Join-Path $engineRoot 'Engine\Binaries\Win64\UnrealEditor.exe'

    if (Test-Path -LiteralPath $candidateCmd -PathType Leaf) {
        return $candidateCmd
    }
    if (Test-Path -LiteralPath $candidateEditor -PathType Leaf) {
        return $candidateEditor
    }

    return $candidateCmd
}

function Resolve-GenerationProfiles {
    param(
        [Parameter(Mandatory = $false)][object]$Config,
        [Parameter(Mandatory = $false)][string]$CliLyraRoot,
        [Parameter(Mandatory = $false)][string]$CliUEEngineRoot,
        [Parameter(Mandatory = $false)][string]$CliEditorCmdPath,
        [Parameter(Mandatory = $false)][string]$CliGoldenRoot,
        [Parameter(Mandatory = $true)][string]$ConfigPathForError
    )

    $profiles = New-Object System.Collections.Generic.List[object]

    if (-not [string]::IsNullOrWhiteSpace($CliLyraRoot)) {
        $profiles.Add([pscustomobject]@{
                Name         = 'cli'
                LyraRoot     = $CliLyraRoot
                UEEngineRoot = $(if (-not [string]::IsNullOrWhiteSpace($CliUEEngineRoot)) { $CliUEEngineRoot } else { [string](Get-ConfigValue -Config $Config -Name 'ueEngineRoot') })
                EditorCmdPath = $(if (-not [string]::IsNullOrWhiteSpace($CliEditorCmdPath)) { $CliEditorCmdPath } else { [string](Get-ConfigValue -Config $Config -Name 'editorCmdPath') })
                GoldenRoot   = $(if (-not [string]::IsNullOrWhiteSpace($CliGoldenRoot)) { $CliGoldenRoot } else { [string](Get-ConfigValue -Config $Config -Name 'goldenRoot') })
            })
        return @($profiles.ToArray())
    }

    $engines = Get-ConfigValue -Config $Config -Name 'engines'
    if ($null -ne $engines) {
        $entries = @($engines.PSObject.Properties | Sort-Object Name)
        if ($entries.Count -eq 0) {
            throw "engines in config is empty ($ConfigPathForError)."
        }

        foreach ($entry in $entries) {
            $engineName = [string]$entry.Name
            $engineCfg = $entry.Value

            $profileLyraRoot = [string](Get-ConfigValue -Config $engineCfg -Name 'lyraRoot')
            if ([string]::IsNullOrWhiteSpace($profileLyraRoot)) {
                $profileLyraRoot = [string](Get-ConfigValue -Config $Config -Name 'lyraRoot')
            }
            if ([string]::IsNullOrWhiteSpace($profileLyraRoot)) {
                throw "engines.$engineName.lyraRoot is required ($ConfigPathForError)."
            }

            $profileUEEngineRoot = $(if (-not [string]::IsNullOrWhiteSpace($CliUEEngineRoot)) { $CliUEEngineRoot } else { [string](Get-ConfigValue -Config $engineCfg -Name 'ueEngineRoot') })
            if ([string]::IsNullOrWhiteSpace($profileUEEngineRoot)) {
                $profileUEEngineRoot = [string](Get-ConfigValue -Config $Config -Name 'ueEngineRoot')
            }

            $profileEditorCmdPath = $(if (-not [string]::IsNullOrWhiteSpace($CliEditorCmdPath)) { $CliEditorCmdPath } else { [string](Get-ConfigValue -Config $engineCfg -Name 'editorCmdPath') })
            if ([string]::IsNullOrWhiteSpace($profileEditorCmdPath)) {
                $profileEditorCmdPath = [string](Get-ConfigValue -Config $Config -Name 'editorCmdPath')
            }

            $profileGoldenRoot = $(if (-not [string]::IsNullOrWhiteSpace($CliGoldenRoot)) { $CliGoldenRoot } else { [string](Get-ConfigValue -Config $engineCfg -Name 'goldenRoot') })
            if ([string]::IsNullOrWhiteSpace($profileGoldenRoot)) {
                $profileGoldenRoot = [string](Get-ConfigValue -Config $Config -Name 'goldenRoot')
            }

            $profiles.Add([pscustomobject]@{
                    Name          = $engineName
                    LyraRoot      = $profileLyraRoot
                    UEEngineRoot  = $profileUEEngineRoot
                    EditorCmdPath = $profileEditorCmdPath
                    GoldenRoot    = $profileGoldenRoot
                })
        }

        return @($profiles.ToArray())
    }

    $flatLyraRoot = [string](Get-ConfigValue -Config $Config -Name 'lyraRoot')
    if ([string]::IsNullOrWhiteSpace($flatLyraRoot)) {
        throw "LyraRoot is required (pass -LyraRoot or set lyraRoot/engines in $ConfigPathForError)."
    }

    $profiles.Add([pscustomobject]@{
            Name          = 'default'
            LyraRoot      = $flatLyraRoot
            UEEngineRoot  = $(if (-not [string]::IsNullOrWhiteSpace($CliUEEngineRoot)) { $CliUEEngineRoot } else { [string](Get-ConfigValue -Config $Config -Name 'ueEngineRoot') })
            EditorCmdPath = $(if (-not [string]::IsNullOrWhiteSpace($CliEditorCmdPath)) { $CliEditorCmdPath } else { [string](Get-ConfigValue -Config $Config -Name 'editorCmdPath') })
            GoldenRoot    = $(if (-not [string]::IsNullOrWhiteSpace($CliGoldenRoot)) { $CliGoldenRoot } else { [string](Get-ConfigValue -Config $Config -Name 'goldenRoot') })
        })

    return @($profiles.ToArray())
}

function Get-SafeFileToken {
    param([Parameter(Mandatory = $true)][string]$Value)

    $token = $Value.Trim()
    if ([string]::IsNullOrWhiteSpace($token)) {
        return 'default'
    }

    $token = [regex]::Replace($token, '[^A-Za-z0-9._-]', '_')
    if ([string]::IsNullOrWhiteSpace($token)) {
        return 'default'
    }

    return $token
}

function Get-LogTail {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $false)][int]$LineCount = 40
    )

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return ''
    }

    $lines = @(Get-Content -LiteralPath $Path -Tail $LineCount)
    return ($lines -join [Environment]::NewLine)
}

function New-CommandletLogPath {
    param(
        [Parameter(Mandatory = $true)][string]$LyraRoot,
        [Parameter(Mandatory = $true)][string]$ProfileName
    )

    $logsDir = Join-Path $LyraRoot 'Saved\Logs'
    if (-not (Test-Path -LiteralPath $logsDir -PathType Container)) {
        New-Item -ItemType Directory -Path $logsDir -Force | Out-Null
    }

    $safeProfileName = Get-SafeFileToken -Value $ProfileName
    $timestamp = Get-Date -Format 'yyyyMMdd-HHmmss'
    return (Join-Path $logsDir ("BPXGenerateFixtures-{0}-{1}.log" -f $safeProfileName, $timestamp))
}

function Assert-CommandletRunSucceeded {
    param(
        [Parameter(Mandatory = $true)][string]$LogPath,
        [Parameter(Mandatory = $true)][string]$ProfileName
    )

    $startupDeadline = (Get-Date).AddSeconds(60)
    while ((Get-Date) -lt $startupDeadline) {
        if (Test-Path -LiteralPath $LogPath -PathType Leaf) {
            break
        }
        Start-Sleep -Seconds 1
    }
    if (-not (Test-Path -LiteralPath $LogPath -PathType Leaf)) {
        throw "Commandlet log file was not generated (profile=$ProfileName): $LogPath"
    }

    $successMarker = 'BPX fixture generation complete.'
    $pluginErrorPattern = 'LogPluginManager:\s+Error:.*BPXFixtureGenerator'
    $completionDeadline = (Get-Date).AddMinutes(30)
    while ((Get-Date) -lt $completionDeadline) {
        $logContent = Get-Content -LiteralPath $LogPath -Raw

        if ($logContent -match $pluginErrorPattern) {
            $tail = Get-LogTail -Path $LogPath
            throw "Plugin load failure detected in commandlet log (profile=$ProfileName): $LogPath`nLast log lines:`n$tail"
        }

        if ($logContent -match [regex]::Escape($successMarker)) {
            return
        }

        if ($logContent -match 'Log file closed') {
            $tail = Get-LogTail -Path $LogPath
            throw "Commandlet log closed without success marker (profile=$ProfileName): $LogPath`nLast log lines:`n$tail"
        }

        Start-Sleep -Seconds 1
    }

    $tail = Get-LogTail -Path $LogPath
    throw "Timed out waiting for commandlet completion marker (profile=$ProfileName): $LogPath`nLast log lines:`n$tail"
}

function Write-BPXPluginModulesManifest {
    param(
        [Parameter(Mandatory = $true)][string]$LyraRoot
    )

    $lyraModulesPath = Join-Path $LyraRoot 'Binaries\Win64\UnrealEditor.modules'
    if (-not (Test-Path -LiteralPath $lyraModulesPath -PathType Leaf)) {
        throw "Project module manifest not found: $lyraModulesPath"
    }

    $lyraModules = Get-Content -LiteralPath $lyraModulesPath -Raw | ConvertFrom-Json
    $buildId = [string]$lyraModules.BuildId
    if ([string]::IsNullOrWhiteSpace($buildId)) {
        throw "BuildId is missing in project module manifest: $lyraModulesPath"
    }

    $pluginBinariesDir = Join-Path $LyraRoot 'Plugins\BPXFixtureGenerator\Binaries\Win64'
    if (-not (Test-Path -LiteralPath $pluginBinariesDir -PathType Container)) {
        throw "Plugin binaries directory not found: $pluginBinariesDir"
    }

    $pluginDllPath = Join-Path $pluginBinariesDir 'UnrealEditor-BPXFixtureGenerator.dll'
    if (-not (Test-Path -LiteralPath $pluginDllPath -PathType Leaf)) {
        throw "Plugin module DLL not found: $pluginDllPath"
    }

    $pluginModulesPath = Join-Path $pluginBinariesDir 'UnrealEditor.modules'
    $manifest = [ordered]@{
        BuildId = $buildId
        Modules = [ordered]@{
            BPXFixtureGenerator = 'UnrealEditor-BPXFixtureGenerator.dll'
        }
    }
    $json = $manifest | ConvertTo-Json -Depth 5
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($pluginModulesPath, $json, $utf8NoBom)
}

function Invoke-FixtureGeneration {
    param(
        [Parameter(Mandatory = $true)][object]$Profile,
        [Parameter(Mandatory = $true)][string]$BpxRepoRoot,
        [Parameter(Mandatory = $true)][string]$Scope,
        [Parameter(Mandatory = $false)][string]$Include,
        [Parameter(Mandatory = $true)][bool]$Force,
        [Parameter(Mandatory = $true)][bool]$SkipEditorBuild,
        [Parameter(Mandatory = $true)][string]$SyncScript
    )

    $profileName = [string]$Profile.Name
    $profileLyraRoot = [string]$Profile.LyraRoot
    $profileUEEngineRoot = [string]$Profile.UEEngineRoot
    $profileEditorCmdPath = [string]$Profile.EditorCmdPath
    $profileGoldenRoot = [string]$Profile.GoldenRoot

    if (-not (Test-WindowsOrUncPath -Path $profileLyraRoot)) {
        throw "LyraRoot must be a Windows drive path or UNC path. Input: $profileLyraRoot"
    }
    if (-not [string]::IsNullOrWhiteSpace($profileUEEngineRoot) -and (-not (Test-WindowsOrUncPath -Path $profileUEEngineRoot))) {
        throw "UEEngineRoot must be a Windows drive path or UNC path. Input: $profileUEEngineRoot"
    }
    if (-not [string]::IsNullOrWhiteSpace($profileGoldenRoot) -and (-not (Test-WindowsOrUncPath -Path $profileGoldenRoot))) {
        throw "GoldenRoot must be a Windows drive path or UNC path. Input: $profileGoldenRoot"
    }

    Write-Host ""
    Write-Host "=== BPX fixture generation profile: $profileName ==="
    Write-Host "LyraRoot: $profileLyraRoot"

    $syncArgs = @{ LyraRoot = $profileLyraRoot }
    if ($Force) {
        $syncArgs['Force'] = $true
    }
    & $SyncScript @syncArgs

    $uprojectPath = Join-Path $profileLyraRoot 'Lyra.uproject'
    if (-not (Test-Path -LiteralPath $uprojectPath -PathType Leaf)) {
        throw "Lyra project file not found: $uprojectPath"
    }

    $resolvedEngineRoot = $profileUEEngineRoot
    if ([string]::IsNullOrWhiteSpace($resolvedEngineRoot)) {
        if (-not [string]::IsNullOrWhiteSpace($profileEditorCmdPath)) {
            $editorBinDir = Split-Path -Parent $profileEditorCmdPath
            $resolvedEngineRoot = (Resolve-Path (Join-Path $editorBinDir '..\..')).Path
        }
        else {
            $resolvedEngineRoot = (Resolve-Path (Join-Path $profileLyraRoot '..\..\..')).Path
        }
    }
    $buildBatCandidates = @(
        (Join-Path $resolvedEngineRoot 'Engine\Build\BatchFiles\Build.bat'),
        (Join-Path $resolvedEngineRoot 'Build\BatchFiles\Build.bat')
    )
    $buildBatPath = $buildBatCandidates[0]
    foreach ($candidate in $buildBatCandidates) {
        if (Test-Path -LiteralPath $candidate -PathType Leaf) {
            $buildBatPath = $candidate
            break
        }
    }

    if (-not (Test-Path -LiteralPath $buildBatPath -PathType Leaf)) {
        throw "Build.bat not found. Checked: $($buildBatCandidates -join ', ')"
    }

    $pluginDllPath = Join-Path $profileLyraRoot 'Plugins\BPXFixtureGenerator\Binaries\Win64\UnrealEditor-BPXFixtureGenerator.dll'

    if (-not $SkipEditorBuild) {
        Write-Host 'Building BPXFixtureGenerator module (non-interactive)...'
        $buildArgs = @(
            'LyraEditor',
            'Win64',
            'Development',
            "-Project=$uprojectPath",
            '-Module=BPXFixtureGenerator',
            '-NoUBTMakefiles',
            '-WaitMutex',
            '-NoHotReloadFromIDE'
        )
        $buildExitCode = Invoke-BatchFile -BatchPath $buildBatPath -Arguments $buildArgs -WorkingDirectory $resolvedEngineRoot
        if ($buildExitCode -ne 0) {
            throw "Build.bat failed with exit code $buildExitCode"
        }

        if (-not (Test-Path -LiteralPath $pluginDllPath -PathType Leaf)) {
            Write-Host 'Module-only build did not emit plugin DLL; running full LyraEditor build fallback...'
            $fullBuildArgs = @(
                'LyraEditor',
                'Win64',
                'Development',
                "-Project=$uprojectPath",
                '-NoUBTMakefiles',
                '-WaitMutex',
                '-NoHotReloadFromIDE'
            )
            $fullBuildExitCode = Invoke-BatchFile -BatchPath $buildBatPath -Arguments $fullBuildArgs -WorkingDirectory $resolvedEngineRoot
            if ($fullBuildExitCode -ne 0) {
                throw "Full Build.bat fallback failed with exit code $fullBuildExitCode"
            }
        }
    }
    elseif (-not (Test-Path -LiteralPath $pluginDllPath -PathType Leaf)) {
        throw "Plugin module DLL is missing and -SkipEditorBuild was specified: $pluginDllPath"
    }

    Write-BPXPluginModulesManifest -LyraRoot $profileLyraRoot

    $resolvedEditorCmdPath = Resolve-EditorExecutable -LyraRoot $profileLyraRoot -UEEngineRoot $profileUEEngineRoot -EditorCmdPath $profileEditorCmdPath
    if (-not (Test-Path -LiteralPath $resolvedEditorCmdPath -PathType Leaf)) {
        throw "Unreal editor executable not found after build step: $resolvedEditorCmdPath"
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
    if (-not [string]::IsNullOrWhiteSpace($profileGoldenRoot)) {
        $cmdArgs += "-GoldenRoot=$profileGoldenRoot"
    }
    $commandletLogPath = New-CommandletLogPath -LyraRoot $profileLyraRoot -ProfileName $profileName
    $cmdArgs += "-abslog=$commandletLogPath"

    Write-Host "Running BPX fixture commandlet..."
    Write-Host "  Editor: $resolvedEditorCmdPath"
    Write-Host "  Project: $uprojectPath"
    Write-Host "  Scope: $Scope"
    if (-not [string]::IsNullOrWhiteSpace($Include)) {
        Write-Host "  Include: $Include"
    }

    & $resolvedEditorCmdPath @cmdArgs
    $exitCode = Get-LastExitCodeOrZero
    if ($exitCode -ne 0) {
        throw "Commandlet failed with exit code $exitCode (profile=$profileName, log=$commandletLogPath)"
    }

    Assert-CommandletRunSucceeded -LogPath $commandletLogPath -ProfileName $profileName
    Write-Host "BPX fixture commandlet completed successfully (profile=$profileName)."
    Write-Host "  Commandlet log: $commandletLogPath"
}

function Invoke-FixtureGenerationParallel {
    param(
        [Parameter(Mandatory = $true)][object[]]$Profiles,
        [Parameter(Mandatory = $true)][string]$ScriptPath,
        [Parameter(Mandatory = $true)][string]$BpxRepoRoot,
        [Parameter(Mandatory = $true)][string]$Scope,
        [Parameter(Mandatory = $false)][string]$Include,
        [Parameter(Mandatory = $true)][bool]$Force,
        [Parameter(Mandatory = $true)][bool]$SkipEditorBuild
    )

    $jobs = New-Object System.Collections.Generic.List[System.Management.Automation.Job]

    foreach ($profile in $Profiles) {
        $profileName = [string]$profile.Name
        Write-Host "Starting fixture generation job: $profileName"

        $payload = [pscustomobject]@{
            Name            = $profileName
            ScriptPath      = $ScriptPath
            LyraRoot        = [string]$profile.LyraRoot
            BpxRepoRoot     = $BpxRepoRoot
            Scope           = $Scope
            Include         = $Include
            Force           = $Force
            SkipEditorBuild = $SkipEditorBuild
            UEEngineRoot    = [string]$profile.UEEngineRoot
            EditorCmdPath   = [string]$profile.EditorCmdPath
            GoldenRoot      = [string]$profile.GoldenRoot
        }

        $job = Start-Job -Name ("bpx-gen-" + $profileName) -ScriptBlock {
            param([Parameter(Mandatory = $true)][object]$Payload)

            $psArgs = @(
                '-NoProfile',
                '-ExecutionPolicy', 'Bypass',
                '-File', [string]$Payload.ScriptPath,
                '-LyraRoot', [string]$Payload.LyraRoot,
                '-BpxRepoRoot', [string]$Payload.BpxRepoRoot,
                '-Scope', [string]$Payload.Scope
            )

            if (-not [string]::IsNullOrWhiteSpace([string]$Payload.Include)) {
                $psArgs += @('-Include', [string]$Payload.Include)
            }
            if ([bool]$Payload.Force) {
                $psArgs += '-Force'
            }
            if ([bool]$Payload.SkipEditorBuild) {
                $psArgs += '-SkipEditorBuild'
            }
            if (-not [string]::IsNullOrWhiteSpace([string]$Payload.UEEngineRoot)) {
                $psArgs += @('-UEEngineRoot', [string]$Payload.UEEngineRoot)
            }
            if (-not [string]::IsNullOrWhiteSpace([string]$Payload.EditorCmdPath)) {
                $psArgs += @('-EditorCmdPath', [string]$Payload.EditorCmdPath)
            }
            if (-not [string]::IsNullOrWhiteSpace([string]$Payload.GoldenRoot)) {
                $psArgs += @('-GoldenRoot', [string]$Payload.GoldenRoot)
            }

            & powershell.exe @psArgs
            $exitCode = 0
            $lastExitCodeVar = Get-Variable -Name LASTEXITCODE -ErrorAction SilentlyContinue
            if ($null -ne $lastExitCodeVar) {
                $exitCode = [int]$lastExitCodeVar.Value
            }
            if ($exitCode -ne 0) {
                throw "profile $([string]$Payload.Name) failed with exit code $exitCode"
            }

            Write-Output "profile $([string]$Payload.Name) completed."
        } -ArgumentList $payload

        $jobs.Add($job) | Out-Null
    }

    if ($jobs.Count -eq 0) {
        return
    }

    Wait-Job -Job ($jobs.ToArray()) | Out-Null

    $failedJobs = New-Object System.Collections.Generic.List[string]
    foreach ($job in $jobs) {
        Write-Host ""
        Write-Host "=== Profile job log: $($job.Name) ==="
        Receive-Job -Job $job -ErrorAction Continue

        if ($job.State -ne 'Completed') {
            $failedJobs.Add($job.Name) | Out-Null
        }
    }

    Remove-Job -Job ($jobs.ToArray()) -Force

    if ($failedJobs.Count -gt 0) {
        throw "Fixture generation failed for profile job(s): $($failedJobs -join ', ')"
    }
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$defaultConfigPath = Join-Path $scriptDir 'local-fixtures.config.json'
$config = Read-LocalConfig -DefaultPath $defaultConfigPath -SpecifiedPath $ConfigPath
$effectiveConfigPath = $ConfigPath

$Scope = Convert-ToNormalizedCsvString -Value $Scope
$Include = Convert-ToNormalizedCsvString -Value $Include

if ([string]::IsNullOrWhiteSpace($effectiveConfigPath)) {
    $effectiveConfigPath = $defaultConfigPath
}

if ([string]::IsNullOrWhiteSpace($BpxRepoRoot)) {
    $BpxRepoRoot = [string](Get-ConfigValue -Config $config -Name 'bpxRepoRoot')
}
if ([string]::IsNullOrWhiteSpace($BpxRepoRoot)) {
    $BpxRepoRoot = (Resolve-Path (Join-Path $scriptDir '..')).Path
}
if (-not (Test-WindowsOrUncPath -Path $BpxRepoRoot)) {
    throw "BpxRepoRoot must be a Windows drive path or UNC path. Input: $BpxRepoRoot"
}

if (($Scope -eq '1,2') -and (-not $PSBoundParameters.ContainsKey('Scope'))) {
    $configuredScope = [string](Get-ConfigValue -Config $config -Name 'scope')
    if (-not [string]::IsNullOrWhiteSpace($configuredScope)) {
        $Scope = $configuredScope
    }
}

if ([string]::IsNullOrWhiteSpace($Include) -and (-not $PSBoundParameters.ContainsKey('Include'))) {
    $configuredInclude = [string](Get-ConfigValue -Config $config -Name 'include')
    if (-not [string]::IsNullOrWhiteSpace($configuredInclude)) {
        $Include = $configuredInclude
    }
}

if (-not $PSBoundParameters.ContainsKey('SkipEditorBuild')) {
    $configuredSkipEditorBuild = Get-ConfigValue -Config $config -Name 'skipEditorBuild'
    if ($null -ne $configuredSkipEditorBuild) {
        $SkipEditorBuild = [bool]$configuredSkipEditorBuild
    }
}

$syncScript = Join-Path $scriptDir 'sync-bpx-plugin.ps1'
if (-not (Test-Path -LiteralPath $syncScript -PathType Leaf)) {
    throw "Sync script not found: $syncScript"
}

$profiles = @(Resolve-GenerationProfiles -Config $config -CliLyraRoot $LyraRoot -CliUEEngineRoot $UEEngineRoot -CliEditorCmdPath $EditorCmdPath -CliGoldenRoot $GoldenRoot -ConfigPathForError $effectiveConfigPath)
if ($profiles.Count -eq 0) {
    throw 'No generation profile resolved.'
}

$duplicateLyraRoots = @($profiles | Group-Object -Property LyraRoot | Where-Object { $_.Count -gt 1 })
if ($duplicateLyraRoots.Count -gt 0) {
    $duplicates = ($duplicateLyraRoots | ForEach-Object { $_.Name }) -join ', '
    throw "Duplicate lyraRoot values are not supported for concurrent generation: $duplicates"
}

$executed = $profiles.Count
if ($profiles.Count -eq 1) {
    Invoke-FixtureGeneration -Profile $profiles[0] -BpxRepoRoot $BpxRepoRoot -Scope $Scope -Include $Include -Force ([bool]$Force) -SkipEditorBuild ([bool]$SkipEditorBuild) -SyncScript $syncScript
}
elseif (Should-RunProfilesSequentially -Profiles $profiles -BpxRepoRoot $BpxRepoRoot) {
    Write-Host "Running fixture generation sequentially because WSL UNC paths are in use."
    foreach ($profile in $profiles) {
        Invoke-FixtureGeneration -Profile $profile -BpxRepoRoot $BpxRepoRoot -Scope $Scope -Include $Include -Force ([bool]$Force) -SkipEditorBuild ([bool]$SkipEditorBuild) -SyncScript $syncScript
    }
}
else {
    Write-Host "Launching fixture generation for $($profiles.Count) profiles in parallel..."
    Invoke-FixtureGenerationParallel -Profiles $profiles -ScriptPath $MyInvocation.MyCommand.Path -BpxRepoRoot $BpxRepoRoot -Scope $Scope -Include $Include -Force ([bool]$Force) -SkipEditorBuild ([bool]$SkipEditorBuild)
}

Write-Host ""
Write-Host "Completed fixture generation for $executed profile(s)."
