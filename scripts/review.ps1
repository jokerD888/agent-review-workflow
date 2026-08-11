[CmdletBinding()]
param(
    [string]$BaseBranch,
    [switch]$OpenVSCode
)

$ErrorActionPreference = 'Stop'

function Invoke-Git {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)
    & git @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Git command failed: git $($Arguments -join ' ')"
    }
}

Invoke-Git rev-parse --show-toplevel | Out-Null

if (-not $BaseBranch) {
    foreach ($candidate in 'main', 'master') {
        & git show-ref --verify --quiet "refs/heads/$candidate"
        if ($LASTEXITCODE -eq 0) {
            $BaseBranch = $candidate
            break
        }
    }
}

if (-not $BaseBranch) {
    throw 'Could not find main or master. Pass -BaseBranch explicitly.'
}

Invoke-Git merge-base $BaseBranch HEAD | Out-Null
Write-Host "Reviewing changes introduced by HEAD against $BaseBranch"
Write-Host "`n=== Diff stat ==="
Invoke-Git diff --stat "$BaseBranch...HEAD"
Write-Host "`n=== Checkpoint commits ==="
Invoke-Git log --reverse --oneline "$BaseBranch..HEAD"
Write-Host "`n=== Whitespace check ==="
& git diff --check "$BaseBranch...HEAD"
if ($LASTEXITCODE -ne 0) {
    Write-Warning 'Whitespace errors found. Review them before merge.'
}

if ($OpenVSCode) {
    $code = Get-Command code -ErrorAction SilentlyContinue
    if ($code) {
        Start-Process $code.Source -ArgumentList '.'
    } else {
        Write-Warning 'VS Code command `code` is not on PATH. Open this repository in VS Code manually.'
    }
}
