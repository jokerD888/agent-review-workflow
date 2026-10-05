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
    $repoRoot = Split-Path -Parent $scriptDir

    # Check dist directory first (release package)
    $distDir = Join-Path $repoRoot 'dist\agent-review-workflow'
    if (Test-Path -LiteralPath (Join-Path $distDir 'SKILL.md')) {
        return $distDir
    }

    # Check source tree skills/agent-review-workflow
    $sourceDir = Join-Path $repoRoot 'skills\agent-review-workflow'
    if (Test-Path -LiteralPath (Join-Path $sourceDir 'SKILL.md')) {
        return $sourceDir
    }

    throw "Could not locate agent-review-workflow skill source files."
}

function Get-TargetPaths([string]$ScopeMode, [string]$HostName, [string]$RepoPath) {
    $userHome = $env:USERPROFILE
    $targets = [System.Collections.Generic.List[string]]::new()

    if ($ScopeMode -eq 'global') {
        $pathsByHost = @{
            'codex'       = if ($env:CODEX_HOME) { Join-Path $env:CODEX_HOME 'skills\agent-review-workflow' } else { Join-Path $userHome '.codex\skills\agent-review-workflow' }
            'claude'      = Join-Path $userHome '.claude\skills\agent-review-workflow'
            'opencode'    = if ($env:XDG_CONFIG_HOME) { Join-Path $env:XDG_CONFIG_HOME 'opencode\skills\agent-review-workflow' } else { Join-Path $userHome '.config\opencode\skills\agent-review-workflow' }
            'antigravity' = Join-Path $userHome '.gemini\antigravity\skills\agent-review-workflow'
            'generic'     = Join-Path $userHome '.agents\skills\agent-review-workflow'
        }

        if ($HostName -eq 'all') {
            foreach ($key in 'antigravity', 'codex', 'claude', 'opencode', 'generic') {
                $targets.Add($pathsByHost[$key])
            }
        } else {
            $targets.Add($pathsByHost[$HostName])
        }
    } else {
        $resolvedRepo = (Resolve-Path -LiteralPath $RepoPath).Path
        $pathsByHost = @{
            'codex'       = Join-Path $resolvedRepo '.codex\skills\agent-review-workflow'
            'claude'      = Join-Path $resolvedRepo '.claude\skills\agent-review-workflow'
            'opencode'    = Join-Path $resolvedRepo '.opencode\skills\agent-review-workflow'
            'antigravity' = Join-Path $resolvedRepo '.agents\skills\agent-review-workflow'
            'generic'     = Join-Path $resolvedRepo '.agents\skills\agent-review-workflow'
        }

        if ($HostName -eq 'all') {
            # In repo scope, .agents/skills is the universal standard
            $targets.Add($pathsByHost['generic'])
        } else {
            $targets.Add($pathsByHost[$HostName])
        }
    }

    return $targets
}

$sourceSkill = Get-SkillSourceDir
$repoRoot = Split-Path -Parent (Split-Path -Parent $sourceSkill)
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

    # Copy bin if present in sourceSkill
    $srcBin = Join-Path $sourceSkill 'bin'
    $destBin = Join-Path $target 'bin'
    if (Test-Path -LiteralPath $srcBin) {
        New-Item -ItemType Directory -Path $destBin -Force | Out-Null
        Get-ChildItem -LiteralPath $srcBin | Copy-Item -Destination $destBin -Recurse -Force
    } else {
        # Check if local development built binary exists in repo root
        $localExe = Join-Path $repoRoot 'bin\arw.exe'
        if (Test-Path -LiteralPath $localExe) {
            $winAmd64Dir = Join-Path $destBin 'windows-amd64'
            New-Item -ItemType Directory -Path $winAmd64Dir -Force | Out-Null
            Copy-Item -LiteralPath $localExe -Destination (Join-Path $winAmd64Dir 'arw.exe') -Force
        }
    }

    Write-Host "Installed skill to: $target"
}

Write-Host "ARW Skill installation complete."
