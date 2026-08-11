[CmdletBinding()]
param(
    [Parameter(Position = 0)][string]$Command = 'help',
    [Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments
)

$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot

switch ($Command.ToLowerInvariant()) {
    'init' {
        $withReviewer = $Arguments -contains '--with-opencode-reviewer'
        $projectPath = (Get-Location).Path
        & (Join-Path $root 'install.ps1') -ProjectPath $projectPath -InstallOpenCodeReviewer:$withReviewer
    }
    'start' {
        if ($Arguments.Count -eq 0) { throw 'Usage: arw start <task name> [-BaseBranch main]' }
        $taskName = $Arguments[0]
        $remaining = if ($Arguments.Count -gt 1) { $Arguments[1..($Arguments.Count - 1)] } else { @() }
        & (Join-Path $root 'scripts\start-task.ps1') -Name $taskName @remaining
    }
    'review' {
        & (Join-Path $root 'scripts\review.ps1') @Arguments
    }
    'update' {
        & (Join-Path $root 'install.ps1') -Update
    }
    'doctor' {
        $userHomePath = [Environment]::GetFolderPath([Environment+SpecialFolder]::UserProfile)
        $checks = @(
            @{ Name = 'Git'; Path = (Get-Command git -ErrorAction SilentlyContinue).Source },
            @{ Name = 'VS Code'; Path = (Get-Command code -ErrorAction SilentlyContinue).Source },
            @{ Name = 'Codex rules'; Path = (Join-Path ($env:CODEX_HOME ?? (Join-Path $userHomePath '.codex')) 'AGENTS.md') },
            @{ Name = 'Claude Code rules'; Path = (Join-Path $userHomePath '.claude\CLAUDE.md') },
            @{ Name = 'OpenCode rules'; Path = (Join-Path $userHomePath '.config\opencode\AGENTS.md') }
        )
        foreach ($check in $checks) {
            $present = if ($check.Name -in 'Git', 'VS Code') { [bool]$check.Path } else { Test-Path -LiteralPath $check.Path }
            "$(if ($present) { 'OK' } else { 'MISSING' })`t$($check.Name)`t$($check.Path)"
        }
    }
    'help' {
        @'
Usage: arw <command>

  arw init [--with-opencode-reviewer]  Configure the current project.
  arw start <task name> [-BaseBranch main]
                                      Create an AI task branch in the current repository.
  arw review [-BaseBranch main] [-OpenVSCode]
                                      Summarize and open the current task diff.
  arw update                          Download the latest workflow files.
  arw doctor                          Check local prerequisites and rule files.
'@
    }
    default { throw "Unknown command: $Command. Run 'arw help'." }
}
