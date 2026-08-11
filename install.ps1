[CmdletBinding()]
param(
    [string]$ProjectPath,
    [switch]$GlobalOnly,
    [switch]$InstallOpenCodeReviewer,
    [string]$UserHomePath
)

$ErrorActionPreference = 'Stop'

$MarkerStart = '<!-- agent-review-workflow:begin -->'
$MarkerEnd = '<!-- agent-review-workflow:end -->'
$RawBaseUrl = 'https://raw.githubusercontent.com/jokerD888/agent-review-workflow/main'
if (-not $UserHomePath) {
    $UserHomePath = [Environment]::GetFolderPath([Environment+SpecialFolder]::UserProfile)
}

function Get-WorkflowFile {
    param([Parameter(Mandatory = $true)][string]$RelativePath)

    $scriptDirectory = Split-Path -Parent $PSCommandPath
    if ($scriptDirectory) {
        $localPath = Join-Path $scriptDirectory $RelativePath
        if (Test-Path -LiteralPath $localPath) {
            return Get-Content -Raw -LiteralPath $localPath
        }
    }

    $uri = "$RawBaseUrl/$RelativePath"
    return (Invoke-WebRequest -UseBasicParsing -Uri $uri).Content
}

function Set-ManagedBlock {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Content
    )

    $directory = Split-Path -Parent $Path
    if (-not (Test-Path -LiteralPath $directory)) {
        New-Item -ItemType Directory -Path $directory -Force | Out-Null
    }

    $block = "$MarkerStart`r`n$($Content.Trim())`r`n$MarkerEnd"
    $existing = if (Test-Path -LiteralPath $Path) {
        [string](Get-Content -Raw -LiteralPath $Path)
    } else {
        ''
    }

    $pattern = '(?s)' + [regex]::Escape($MarkerStart) + '.*?' + [regex]::Escape($MarkerEnd)
    if ([regex]::IsMatch($existing, $pattern)) {
        $updated = [regex]::Replace($existing, $pattern, [System.Text.RegularExpressions.MatchEvaluator]{ param($match) $block })
    } elseif ([string]::IsNullOrWhiteSpace($existing)) {
        $updated = "$block`r`n"
    } else {
        $updated = "$($existing.TrimEnd())`r`n`r`n$block`r`n"
    }

    Set-Content -LiteralPath $Path -Value $updated -Encoding utf8
    Write-Host "Updated $Path"
}

function Add-ProjectFiles {
    param([Parameter(Mandatory = $true)][string]$Path)

    $resolvedProjectPath = (Resolve-Path -LiteralPath $Path).Path
    $projectRules = Get-WorkflowFile 'templates/AGENTS.md'
    $claudeShim = Get-WorkflowFile 'templates/CLAUDE.md'

    Set-ManagedBlock -Path (Join-Path $resolvedProjectPath 'AGENTS.md') -Content $projectRules
    Set-ManagedBlock -Path (Join-Path $resolvedProjectPath 'CLAUDE.md') -Content $claudeShim

    if ($InstallOpenCodeReviewer) {
        $reviewerPath = Join-Path $resolvedProjectPath '.opencode/agents/review.md'
        if (Test-Path -LiteralPath $reviewerPath) {
            Write-Warning "Skipped existing OpenCode reviewer: $reviewerPath"
        } else {
            $reviewerDirectory = Split-Path -Parent $reviewerPath
            New-Item -ItemType Directory -Path $reviewerDirectory -Force | Out-Null
            Set-Content -LiteralPath $reviewerPath -Value (Get-WorkflowFile 'templates/opencode-reviewer.md') -Encoding utf8
            Write-Host "Created $reviewerPath"
        }
    }
}

if ($GlobalOnly -and $ProjectPath) {
    throw 'Use either -GlobalOnly or -ProjectPath, not both.'
}

$globalRules = Get-WorkflowFile 'rules/global.md'
$codexHomePath = if ($env:CODEX_HOME) { $env:CODEX_HOME } else { Join-Path $UserHomePath '.codex' }

Set-ManagedBlock -Path (Join-Path $codexHomePath 'AGENTS.md') -Content $globalRules
Set-ManagedBlock -Path (Join-Path $UserHomePath '.claude/CLAUDE.md') -Content $globalRules
Set-ManagedBlock -Path (Join-Path $UserHomePath '.config/opencode/AGENTS.md') -Content $globalRules

if ($ProjectPath) {
    Add-ProjectFiles -Path $ProjectPath
}

Write-Host 'Agent Review Workflow installation complete.'
