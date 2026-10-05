[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$testDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent (Split-Path -Parent $testDir)
$wrapperPs1 = Join-Path $repoRoot 'skills\agent-review-workflow\scripts\arw.ps1'

Write-Host "Running wrapper tests..."

# Test 1: Missing binary fails with exit code 127
$tempSkill = Join-Path ([System.IO.Path]::GetTempPath()) ("arw-test-skill-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path (Join-Path $tempSkill 'scripts') -Force | Out-Null
Copy-Item -LiteralPath $wrapperPs1 -Destination (Join-Path $tempSkill 'scripts\arw.ps1')

$proc = Start-Process -FilePath "pwsh" -ArgumentList "-NoProfile", "-File", (Join-Path $tempSkill 'scripts\arw.ps1'), "version" -PassThru -NoNewWindow -Wait -RedirectStandardError (Join-Path $tempSkill 'err.txt')
if ($proc.ExitCode -ne 127) {
    throw "Test 1 failed: Expected exit code 127 for missing binary, got $($proc.ExitCode)"
}
$errContent = Get-Content -Raw -LiteralPath (Join-Path $tempSkill 'err.txt')
if ($errContent -notmatch "not found at") {
    throw "Test 1 failed: Expected 'not found at' in stderr, got: $errContent"
}
Write-Host "PASS: Test 1 - Missing binary returns 127 with clear error"

# Test 2: When platform binary is present, wrapper forwards arguments, stdout, and exit code
$binDir = Join-Path $tempSkill 'bin\windows-amd64'
New-Item -ItemType Directory -Path $binDir -Force | Out-Null

# We can copy our compiled arw.exe to test end-to-end
$compiledArw = Join-Path $repoRoot 'bin\arw.exe'
if (-not (Test-Path -LiteralPath $compiledArw)) {
    & go build -o $compiledArw (Join-Path $repoRoot 'cmd\arw')
}
Copy-Item -LiteralPath $compiledArw -Destination (Join-Path $binDir 'arw.exe')

$outTxt = Join-Path $tempSkill 'out.txt'
$proc2 = Start-Process -FilePath "pwsh" -ArgumentList "-NoProfile", "-File", (Join-Path $tempSkill 'scripts\arw.ps1'), "version", "--json" -PassThru -NoNewWindow -Wait -RedirectStandardOutput $outTxt
if ($proc2.ExitCode -ne 0) {
    throw "Test 2 failed: Expected exit code 0, got $($proc2.ExitCode)"
}
$outContent = Get-Content -Raw -LiteralPath $outTxt
$versionJson = $outContent | ConvertFrom-Json
if ($versionJson.version -ne "0.2.0-dev") {
    throw "Test 2 failed: Expected version 0.2.0-dev, got: $outContent"
}
Write-Host "PASS: Test 2 - Wrapper transparently forwards args, stdout, and exit code"

# Test 3: Wrapper preserves non-zero exit code from CLI
$errTxt2 = Join-Path $tempSkill 'err2.txt'
$proc3 = Start-Process -FilePath "pwsh" -ArgumentList "-NoProfile", "-File", (Join-Path $tempSkill 'scripts\arw.ps1'), "nonexistent-cmd" -PassThru -NoNewWindow -Wait -RedirectStandardError $errTxt2
if ($proc3.ExitCode -eq 0) {
    throw "Test 3 failed: Expected non-zero exit code for invalid command, got 0"
}
Write-Host "PASS: Test 3 - Wrapper preserves non-zero exit code from CLI"

# Cleanup
Remove-Item -LiteralPath $tempSkill -Recurse -Force -ErrorAction SilentlyContinue
Write-Host "ALL WRAPPER TESTS PASSED!"
