[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Name,
    [string]$BaseBranch = 'main'
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

$workingTree = (& git status --porcelain)
if ($workingTree) {
    throw 'Working tree is not clean. Commit, stash, or review existing changes before starting a task branch.'
}

Invoke-Git show-ref --verify --quiet "refs/heads/$BaseBranch"

$slug = $Name.Trim().ToLowerInvariant() -replace '[^a-z0-9]+', '-' -replace '(^-|-$)', ''
if ([string]::IsNullOrWhiteSpace($slug)) {
    throw 'Task name must contain letters or numbers.'
}

$branchName = "ai/$slug"
& git show-ref --verify --quiet "refs/heads/$branchName"
if ($LASTEXITCODE -eq 0) {
    throw "Branch already exists: $branchName"
}
if ($LASTEXITCODE -gt 1) {
    throw "Could not check whether branch exists: $branchName"
}

Invoke-Git switch $BaseBranch
Invoke-Git switch -c $branchName
Write-Host "Ready for AI work on $branchName"
