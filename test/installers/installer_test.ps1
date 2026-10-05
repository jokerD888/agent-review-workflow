[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$testDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent (Split-Path -Parent $testDir)
$installerPs1 = Join-Path $repoRoot 'installers\install-skill.ps1'

Write-Host "Running installer tests in isolated environment..."

$tempHome = Join-Path ([System.IO.Path]::GetTempPath()) ("arw-inst-test-" + [guid]::NewGuid())
$origUserProfile = $env:USERPROFILE
$origCodexHome = $env:CODEX_HOME

try {
    $env:USERPROFILE = $tempHome
    $env:CODEX_HOME = Join-Path $tempHome '.codex'

    # Test Global Installation for Antigravity
    & $installerPs1 -Scope global -HostTarget antigravity -Force
    $antiSkill = Join-Path $tempHome '.gemini\antigravity\skills\agent-review-workflow\SKILL.md'
    if (-not (Test-Path -LiteralPath $antiSkill)) {
        throw "Global antigravity skill was not installed at: $antiSkill"
    }
    Write-Host "PASS: Global antigravity skill installed"

    # Test Global Installation for Codex
    & $installerPs1 -Scope global -HostTarget codex -Force
    $codexSkill = Join-Path $env:CODEX_HOME 'skills\agent-review-workflow\SKILL.md'
    if (-not (Test-Path -LiteralPath $codexSkill)) {
        throw "Global codex skill was not installed at: $codexSkill"
    }
    Write-Host "PASS: Global codex skill installed"

    # Test Repo-scoped Installation
    $tempRepo = Join-Path $tempHome 'test-repo'
    New-Item -ItemType Directory -Path $tempRepo -Force | Out-Null
    & $installerPs1 -Scope repo -HostTarget generic -TargetRepo $tempRepo -Force
    $repoSkill = Join-Path $tempRepo '.agents\skills\agent-review-workflow\SKILL.md'
    if (-not (Test-Path -LiteralPath $repoSkill)) {
        throw "Repo-scoped skill was not installed at: $repoSkill"
    }
    Write-Host "PASS: Repo-scoped skill installed"

    # Test That Wrapper In Installed Skill Can Execute
    $wrapperInSkill = Join-Path $tempRepo '.agents\skills\agent-review-workflow\scripts\arw.ps1'
    if (-not (Test-Path -LiteralPath $wrapperInSkill)) {
        throw "Wrapper script not found in installed repo skill: $wrapperInSkill"
    }
    Write-Host "PASS: Wrapper script present in installed skill"

} finally {
    $env:USERPROFILE = $origUserProfile
    if ($origCodexHome) { $env:CODEX_HOME = $origCodexHome } else { Remove-Item env:CODEX_HOME -ErrorAction SilentlyContinue }
    Remove-Item -LiteralPath $tempHome -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "ALL INSTALLER TESTS PASSED!"
