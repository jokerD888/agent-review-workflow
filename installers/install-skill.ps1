[CmdletBinding()]
param(
    [ValidateSet('global', 'repo')]
    [string]$Scope = 'global',

    [ValidateSet('all', 'antigravity', 'claude', 'codex', 'opencode', 'generic')]
    [string]$HostTarget = 'all',

    [string]$TargetRepo = '.',
    [string]$Version = '',
    [switch]$Force
)

$ErrorActionPreference = 'Stop'

function Get-SkillSourceDir {
    $scriptDir = $PSScriptRoot

    # 1. If script is inside install/ subfolder of a self-contained release package
    $parent = Split-Path -Parent $scriptDir
    if (Test-Path -LiteralPath (Join-Path $parent 'SKILL.md')) {
        return $parent
    }

    # 2. If script is directly in the skill root
    if (Test-Path -LiteralPath (Join-Path $scriptDir 'SKILL.md')) {
        return $scriptDir
    }

    # 3. Check dist directory (release package in repo)
    $distDir = Join-Path $parent 'dist\agent-review-workflow'
    if (Test-Path -LiteralPath (Join-Path $distDir 'SKILL.md')) {
        return $distDir
    }

    # 4. Check source tree skills/agent-review-workflow
    $sourceDir = Join-Path $parent 'skills\agent-review-workflow'
    if (Test-Path -LiteralPath (Join-Path $sourceDir 'SKILL.md')) {
        return $sourceDir
    }

    throw "Could not locate agent-review-workflow skill source files."
}

function Get-TargetPaths([string]$ScopeMode, [string]$HostName, [string]$RepoPath) {
    $userHome = $env:USERPROFILE
    $targets = [System.Collections.Generic.List[string]]::new()

    if ($ScopeMode -eq 'global') {
        $antiOfficial = Join-Path $userHome '.gemini\config\skills\agent-review-workflow'
        $antiFallback = Join-Path $userHome '.gemini\antigravity\skills\agent-review-workflow'
        $codexHome = if ($env:CODEX_HOME) { Join-Path $env:CODEX_HOME 'skills\agent-review-workflow' } else { Join-Path $userHome '.codex\skills\agent-review-workflow' }
        $claudeHome = Join-Path $userHome '.claude\skills\agent-review-workflow'
        $openCodeHome = if ($env:XDG_CONFIG_HOME) { Join-Path $env:XDG_CONFIG_HOME 'opencode\skills\agent-review-workflow' } else { Join-Path $userHome '.config\opencode\skills\agent-review-workflow' }
        $genericHome = Join-Path $userHome '.agents\skills\agent-review-workflow'

        switch ($HostName) {
            'antigravity' {
                $targets.Add($antiOfficial)
                $targets.Add($antiFallback)
            }
            'codex' { $targets.Add($codexHome) }
            'claude' { $targets.Add($claudeHome) }
            'opencode' { $targets.Add($openCodeHome) }
            'generic' { $targets.Add($genericHome) }
            'all' {
                $targets.Add($antiOfficial)
                $targets.Add($antiFallback)
                $targets.Add($codexHome)
                $targets.Add($claudeHome)
                $targets.Add($openCodeHome)
                $targets.Add($genericHome)
            }
        }
    } else {
        $resolvedRepo = (Resolve-Path -LiteralPath $RepoPath).Path
        $antiRepo = Join-Path $resolvedRepo '.agents\skills\agent-review-workflow'
        $claudeRepo = Join-Path $resolvedRepo '.claude\skills\agent-review-workflow'
        $codexRepo = Join-Path $resolvedRepo '.codex\skills\agent-review-workflow'
        $openCodeRepo = Join-Path $resolvedRepo '.opencode\skills\agent-review-workflow'
        $genericRepo = Join-Path $resolvedRepo '.agents\skills\agent-review-workflow'

        switch ($HostName) {
            'antigravity' { $targets.Add($antiRepo) }
            'claude' { $targets.Add($claudeRepo) }
            'codex' { $targets.Add($codexRepo) }
            'opencode' { $targets.Add($openCodeRepo) }
            'generic' { $targets.Add($genericRepo) }
            'all' {
                $targets.Add($antiRepo)
                $targets.Add($claudeRepo)
                $targets.Add($codexRepo)
                $targets.Add($openCodeRepo)
            }
        }
    }

    return $targets
}

$sourceSkill = Get-SkillSourceDir
$repoRoot = Split-Path -Parent (Split-Path -Parent $sourceSkill)

# Detect current platform to verify binary presence
$isWin = $IsWindows -or ($env:OS -eq 'Windows_NT')
$targetOs = if ($isWin) { 'windows' } elseif ($IsLinux) { 'linux' } elseif ($IsMacOS) { 'darwin' } else { 'windows' }
$rawArch = try { [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant() } catch { '' }
$targetArch = switch ($rawArch) {
    'x64' { 'amd64' }
    'arm64' { 'arm64' }
    default { if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' } }
}
$currentPlatform = "$targetOs-$targetArch"
$binExt = if ($isWin) { '.exe' } else { '' }
$binName = "arw$binExt"

$srcBin = Join-Path $sourceSkill 'bin'
$hasPrebuiltBin = (Test-Path -LiteralPath (Join-Path $srcBin $currentPlatform)) -or
                  (Test-Path -LiteralPath (Join-Path $srcBin "$currentPlatform\$binName"))
$localBuiltBin = Join-Path $repoRoot "bin\$binName"
$hasLocalBuiltBin = Test-Path -LiteralPath $localBuiltBin

if (-not $hasPrebuiltBin -and -not $hasLocalBuiltBin) {
    throw "No arw binary found for platform '$currentPlatform'.`nIf installing from source, compile the CLI binary first:`n  go build -o ./bin/$binName ./cmd/arw`nOr install using a pre-packaged release package containing bin/."
}

$targets = Get-TargetPaths -ScopeMode $Scope -HostName $HostTarget -RepoPath $TargetRepo

Write-Host "Installing ARW Skill (Scope: $Scope, Host: $HostTarget)..."

foreach ($target in $targets) {
    if (Test-Path -LiteralPath $target) {
        if (-not $Force) {
            Write-Host "Skill already exists at $target. Use -Force to overwrite."
            continue
        }
        Remove-Item -LiteralPath $target -Recurse -Force
    }

    New-Item -ItemType Directory -Path $target -Force | Out-Null

    # Copy SKILL.md
    Copy-Item -LiteralPath (Join-Path $sourceSkill 'SKILL.md') -Destination (Join-Path $target 'SKILL.md') -Force

    # Copy scripts
    $srcScripts = Join-Path $sourceSkill 'scripts'
    if (Test-Path -LiteralPath $srcScripts) {
        $destScripts = Join-Path $target 'scripts'
        New-Item -ItemType Directory -Path $destScripts -Force | Out-Null
        Get-ChildItem -LiteralPath $srcScripts | Copy-Item -Destination $destScripts -Recurse -Force
    }

    # Copy references
    $srcRefs = Join-Path $sourceSkill 'references'
    if (Test-Path -LiteralPath $srcRefs) {
        $destRefs = Join-Path $target 'references'
        New-Item -ItemType Directory -Path $destRefs -Force | Out-Null
        Get-ChildItem -LiteralPath $srcRefs | Copy-Item -Destination $destRefs -Recurse -Force
    }

    # Copy bin
    $destBin = Join-Path $target 'bin'
    if ($hasPrebuiltBin) {
        New-Item -ItemType Directory -Path $destBin -Force | Out-Null
        Get-ChildItem -LiteralPath $srcBin | Copy-Item -Destination $destBin -Recurse -Force
    } elseif ($hasLocalBuiltBin) {
        $platformDir = Join-Path $destBin $currentPlatform
        New-Item -ItemType Directory -Path $platformDir -Force | Out-Null
        Copy-Item -LiteralPath $localBuiltBin -Destination (Join-Path $platformDir $binName) -Force
    }

    Write-Host "Installed skill to: $target"
}

Write-Host "ARW Skill installation complete."
