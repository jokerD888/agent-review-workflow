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

    # Test Global Installation for Antigravity (checks official config path)
    & $installerPs1 -Scope global -HostTarget antigravity -Force
    $antiOfficial = Join-Path $tempHome '.gemini\config\skills\agent-review-workflow\SKILL.md'
    $antiLegacy = Join-Path $tempHome '.gemini\antigravity\skills\agent-review-workflow\SKILL.md'
    if (-not (Test-Path -LiteralPath $antiOfficial)) {
        throw "Official Antigravity skill path was not installed at: $antiOfficial"
    }
    if (-not (Test-Path -LiteralPath $antiLegacy)) {
        throw "Legacy Antigravity skill path was not installed at: $antiLegacy"
    }
    Write-Host "PASS: Global antigravity official and fallback skills installed"

    # Test Global Installation for Codex
    & $installerPs1 -Scope global -HostTarget codex -Force
    $codexSkill = Join-Path $env:CODEX_HOME 'skills\agent-review-workflow\SKILL.md'
    if (-not (Test-Path -LiteralPath $codexSkill)) {
        throw "Global codex skill was not installed at: $codexSkill"
    }
    Write-Host "PASS: Global codex skill installed"

    # Test Repo-scoped Installation for ALL hosts
    $tempRepo = Join-Path $tempHome 'test-repo'
    New-Item -ItemType Directory -Path $tempRepo -Force | Out-Null
    & $installerPs1 -Scope repo -HostTarget all -TargetRepo $tempRepo -Force

    $agentsSkill = Join-Path $tempRepo '.agents\skills\agent-review-workflow\SKILL.md'
    $claudeSkill = Join-Path $tempRepo '.claude\skills\agent-review-workflow\SKILL.md'
    $codexRepoSkill = Join-Path $tempRepo '.codex\skills\agent-review-workflow\SKILL.md'
    $opencodeSkill = Join-Path $tempRepo '.opencode\skills\agent-review-workflow\SKILL.md'

    if (-not (Test-Path -LiteralPath $agentsSkill)) { throw "Repo skill missing at: $agentsSkill" }
    if (-not (Test-Path -LiteralPath $claudeSkill)) { throw "Claude repo skill missing at: $claudeSkill" }
    if (-not (Test-Path -LiteralPath $codexRepoSkill)) { throw "Codex repo skill missing at: $codexRepoSkill" }
    if (-not (Test-Path -LiteralPath $opencodeSkill)) { throw "OpenCode repo skill missing at: $opencodeSkill" }
    Write-Host "PASS: Repo-scoped skills installed across all host directories (.agents, .claude, .codex, .opencode)"

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
