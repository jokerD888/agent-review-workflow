[CmdletBinding()]
param(
    [string]$Version = 'latest',
    [switch]$InstallExtension,
    [switch]$ConfigureAgents,
    [switch]$Force
)

$ErrorActionPreference = 'Stop'
$Repository = 'jokerD888/agent-review-workflow'
$RuntimeRoot = Join-Path $env:LOCALAPPDATA 'AgentReviewWorkflow'
$BinDirectory = Join-Path $RuntimeRoot 'bin'

function Resolve-ReleaseTag {
    if ($Version -ne 'latest') { return $Version }
    $response = Invoke-WebRequest -UseBasicParsing -MaximumRedirection 5 -Uri "https://github.com/$Repository/releases/latest"
    if ($response.BaseResponse.RequestMessage.RequestUri.AbsolutePath -match '/releases/tag/([^/]+)$') { return $matches[1] }
    $tagMatch = [regex]::Match($response.Content, '/releases/tag/(v[^"?#< /]+)')
    if (-not $tagMatch.Success) { throw 'Could not resolve the latest ARW release tag.' }
    return $tagMatch.Groups[1].Value
}

function Download-Checked([string]$ReleaseBase, [string]$Name, [string]$Destination, [hashtable]$Checksums) {
    Invoke-WebRequest -UseBasicParsing -Uri "$ReleaseBase/$Name" -OutFile $Destination
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $Destination).Hash.ToLowerInvariant()
    if (-not $Checksums.ContainsKey($Name) -or $Checksums[$Name] -ne $actual) { Remove-Item -LiteralPath $Destination -Force; throw "SHA-256 verification failed for $Name." }
}

function Set-ManagedRules([string]$Path, [string]$RulesPath) {
    $begin = '<!-- agent-review-workflow:begin -->'; $end = '<!-- agent-review-workflow:end -->'
    $directory = Split-Path -Parent $Path
    New-Item -ItemType Directory -Path $directory -Force | Out-Null
    $rules = (Get-Content -Raw -LiteralPath $RulesPath).Trim()
    $block = "$begin`r`n$rules`r`n$end"
    $existing = if (Test-Path -LiteralPath $Path) { Get-Content -Raw -LiteralPath $Path } else { '' }
    $pattern = '(?s)' + [regex]::Escape($begin) + '.*?' + [regex]::Escape($end)
    $updated = if ([regex]::IsMatch($existing, $pattern)) { [regex]::Replace($existing, $pattern, $block) } elseif ([string]::IsNullOrWhiteSpace($existing)) { "$block`r`n" } else { "$($existing.TrimEnd())`r`n`r`n$block`r`n" }
    Set-Content -LiteralPath $Path -Value $updated -Encoding utf8
}

function Set-AgentMcpConfig([string]$RulesPath) {
    $mcpPath = Join-Path $BinDirectory 'arw-mcp.exe'
    $codexConfig = Join-Path $env:USERPROFILE '.codex\config.toml'
    Set-ManagedRules (Join-Path $env:USERPROFILE '.codex\AGENTS.md') $RulesPath
    Set-ManagedRules (Join-Path $env:USERPROFILE '.claude\CLAUDE.md') $RulesPath
    Set-ManagedRules (Join-Path $env:USERPROFILE '.config\opencode\AGENTS.md') $RulesPath
    {
        New-Item -ItemType Directory -Path (Split-Path -Parent $codexConfig) -Force | Out-Null
        $begin = '# agent-review-workflow:mcp:begin'; $end = '# agent-review-workflow:mcp:end'
        $block = "$begin`r`n[mcp_servers.arw]`r`ncommand = '$mcpPath'`r`nstartup_timeout_sec = 30`r`n$end"
        $existing = if (Test-Path -LiteralPath $codexConfig) { Get-Content -Raw -LiteralPath $codexConfig } else { '' }
        $pattern = '(?s)' + [regex]::Escape($begin) + '.*?' + [regex]::Escape($end)
        $updated = if ([regex]::IsMatch($existing, $pattern)) { [regex]::Replace($existing, $pattern, $block) } else { "$($existing.TrimEnd())`r`n`r`n$block`r`n" }
        Set-Content -LiteralPath $codexConfig -Value $updated -Encoding utf8
    }
    if (Get-Command claude -ErrorAction SilentlyContinue) { & claude mcp add --scope user arw -- $mcpPath | Out-Host }
    if (Get-Command opencode -ErrorAction SilentlyContinue) {
        $configPath = Join-Path $env:USERPROFILE '.config\opencode\opencode.json'
        $config = if (Test-Path -LiteralPath $configPath) { Get-Content -Raw -LiteralPath $configPath | ConvertFrom-Json -AsHashtable } else { @{} }
        if (-not $config.ContainsKey('mcp')) { $config.mcp = @{} }
        $majorVersion = [int](((& opencode --version) -split '\.')[0])
        $server = @{ type = 'local'; command = @($mcpPath) }
        if ($majorVersion -ge 2) {
            if (-not $config.mcp.ContainsKey('servers')) { $config.mcp.servers = @{} }
            $server.timeout = @{ startup = 30000 }
            $config.mcp.servers.arw = $server
        } else {
            if ($config.mcp.ContainsKey('servers')) { $config.mcp.Remove('servers') }
            $server.enabled = $true
            $server.timeout = 30000
            $config.mcp.arw = $server
        }
        New-Item -ItemType Directory -Path (Split-Path -Parent $configPath) -Force | Out-Null
        $config | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $configPath -Encoding utf8
    }
}

$tag = Resolve-ReleaseTag
$releaseBase = "https://github.com/$Repository/releases/download/$tag"
$temporary = New-Item -ItemType Directory -Path (Join-Path ([IO.Path]::GetTempPath()) ('arw-install-' + [guid]::NewGuid()))
try {
    $checksumsPath = Join-Path $temporary 'checksums.txt'
    Invoke-WebRequest -UseBasicParsing -Uri "$releaseBase/checksums.txt" -OutFile $checksumsPath
    $checksums = @{}
    Get-Content -LiteralPath $checksumsPath | ForEach-Object { if ($_ -match '^([a-fA-F0-9]{64})\s+\*?(.+)$') { $checksums[$matches[2]] = $matches[1].ToLowerInvariant() } }
    $rulesPath = Join-Path $temporary 'arw-global-rules.md'
    Download-Checked $releaseBase 'arw-global-rules.md' $rulesPath $checksums
    New-Item -ItemType Directory -Path $BinDirectory -Force | Out-Null
    foreach ($name in 'arw_windows_amd64.exe', 'arw-mcp_windows_amd64.exe') {
        $target = Join-Path $BinDirectory $name
        if ((Test-Path -LiteralPath $target) -and -not $Force) { throw "$target already exists; pass -Force to replace it." }
        Download-Checked $releaseBase $name $target $checksums
    }
    Copy-Item -LiteralPath (Join-Path $BinDirectory 'arw_windows_amd64.exe') -Destination (Join-Path $BinDirectory 'arw.exe') -Force
    Copy-Item -LiteralPath (Join-Path $BinDirectory 'arw-mcp_windows_amd64.exe') -Destination (Join-Path $BinDirectory 'arw-mcp.exe') -Force
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $BinDirectory) { [Environment]::SetEnvironmentVariable('Path', "$userPath;$BinDirectory", 'User'); Write-Host "Added $BinDirectory to user PATH; open a new terminal." }
    if ($InstallExtension) {
        $vsix = "agent-review-workflow-$tag.vsix"
        $vsixPath = Join-Path $temporary $vsix
        Download-Checked $releaseBase $vsix $vsixPath $checksums
        & code --install-extension $vsixPath --force
    }
    if ($ConfigureAgents) { Set-AgentMcpConfig $rulesPath }
    Write-Host "Installed ARW $tag in $RuntimeRoot. Run 'arw doctor' from a repository."
} finally { Remove-Item -LiteralPath $temporary -Recurse -Force -ErrorAction SilentlyContinue }
