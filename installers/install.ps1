[CmdletBinding()]
param(
    [string]$Version = 'latest',
    [switch]$InstallExtension,
    [switch]$Force
)

$ErrorActionPreference = 'Stop'
$Repository = 'jokerD888/agent-review-workflow'
$RuntimeRoot = Join-Path $env:LOCALAPPDATA 'AgentReviewWorkflow'
$BinDirectory = Join-Path $RuntimeRoot 'bin'

function Get-Release {
    $uri = if ($Version -eq 'latest') { "https://api.github.com/repos/$Repository/releases/latest" } else { "https://api.github.com/repos/$Repository/releases/tags/$Version" }
    return Invoke-RestMethod -Headers @{ Accept = 'application/vnd.github+json' } -Uri $uri
}

function Get-Asset([object]$Release, [string]$Name) {
    $asset = @($Release.assets | Where-Object name -eq $Name)[0]
    if (-not $asset) { throw "Release $($Release.tag_name) does not contain $Name." }
    return $asset
}

function Download-Checked([object]$Release, [string]$Name, [string]$Destination, [hashtable]$Checksums) {
    $asset = Get-Asset $Release $Name
    Invoke-WebRequest -UseBasicParsing -Uri $asset.browser_download_url -OutFile $Destination
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $Destination).Hash.ToLowerInvariant()
    if (-not $Checksums.ContainsKey($Name) -or $Checksums[$Name] -ne $actual) { Remove-Item -LiteralPath $Destination -Force; throw "SHA-256 verification failed for $Name." }
}

$release = Get-Release
$checksumsAsset = Get-Asset $release 'checksums.txt'
$temporary = New-Item -ItemType Directory -Path (Join-Path ([IO.Path]::GetTempPath()) ('arw-install-' + [guid]::NewGuid()))
try {
    $checksumsPath = Join-Path $temporary 'checksums.txt'
    Invoke-WebRequest -UseBasicParsing -Uri $checksumsAsset.browser_download_url -OutFile $checksumsPath
    $checksums = @{}
    Get-Content -LiteralPath $checksumsPath | ForEach-Object { if ($_ -match '^([a-fA-F0-9]{64})\s+\*?(.+)$') { $checksums[$matches[2]] = $matches[1].ToLowerInvariant() } }
    New-Item -ItemType Directory -Path $BinDirectory -Force | Out-Null
    foreach ($name in 'arw_windows_amd64.exe', 'arw-mcp_windows_amd64.exe') {
        $target = Join-Path $BinDirectory $name
        if ((Test-Path -LiteralPath $target) -and -not $Force) { throw "$target already exists; pass -Force to replace it." }
        Download-Checked $release $name $target $checksums
    }
    Copy-Item -LiteralPath (Join-Path $BinDirectory 'arw_windows_amd64.exe') -Destination (Join-Path $BinDirectory 'arw.exe') -Force
    Copy-Item -LiteralPath (Join-Path $BinDirectory 'arw-mcp_windows_amd64.exe') -Destination (Join-Path $BinDirectory 'arw-mcp.exe') -Force
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $BinDirectory) { [Environment]::SetEnvironmentVariable('Path', "$userPath;$BinDirectory", 'User'); Write-Host "Added $BinDirectory to user PATH; open a new terminal." }
    if ($InstallExtension) {
        $vsix = @($release.assets | Where-Object name -like 'agent-review-workflow-*.vsix')[0]
        if (-not $vsix) { throw 'The release has no VS Code extension asset.' }
        $vsixPath = Join-Path $temporary $vsix.name
        Download-Checked $release $vsix.name $vsixPath $checksums
        & code --install-extension $vsixPath --force
    }
    Write-Host "Installed ARW $($release.tag_name) in $RuntimeRoot. Run 'arw doctor' from a repository."
} finally { Remove-Item -LiteralPath $temporary -Recurse -Force -ErrorAction SilentlyContinue }
