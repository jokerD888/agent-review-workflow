$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$SkillRoot = Split-Path -Parent $ScriptDir

# 1. Determine OS
$os = $null
$ext = ''
if ($IsWindows -or ($env:OS -eq 'Windows_NT')) {
    $os = 'windows'
    $ext = '.exe'
} elseif ($IsLinux) {
    $os = 'linux'
} elseif ($IsMacOS) {
    $os = 'darwin'
} else {
    $uname = (& uname -s 2>$null)
    switch -Regex ($uname) {
        'Linux' { $os = 'linux' }
        'Darwin' { $os = 'darwin' }
        'MINGW|MSYS|CYGWIN' { $os = 'windows'; $ext = '.exe' }
        default {
            [Console]::Error.WriteLine("arw: unsupported operating system '$uname'")
            exit 1
        }
    }
}

# 2. Determine Architecture
$arch = $null
$rawArch = ''
try {
    $rawArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
} catch {
    $rawArch = ''
}

switch ($rawArch.ToLowerInvariant()) {
    'x64' { $arch = 'amd64' }
    'arm64' { $arch = 'arm64' }
    default {
        $envArch = $env:PROCESSOR_ARCHITECTURE
        switch ($envArch) {
            'AMD64' { $arch = 'amd64' }
            'ARM64' { $arch = 'arm64' }
            default {
                $m = (& uname -m 2>$null)
                switch ($m) {
                    'x86_64' { $arch = 'amd64' }
                    'amd64' { $arch = 'amd64' }
                    'aarch64' { $arch = 'arm64' }
                    'arm64' { $arch = 'arm64' }
                    default {
                        [Console]::Error.WriteLine("arw: unsupported architecture: $rawArch / $envArch / $m")
                        exit 1
                    }
                }
            }
        }
    }
}

$platform = "$os-$arch"
$binName = "arw$ext"
$target = Join-Path $SkillRoot (Join-Path 'bin' (Join-Path $platform $binName))

if (-not (Test-Path -LiteralPath $target)) {
    [Console]::Error.WriteLine("arw binary for platform '$platform' not found at: $target")
    exit 127
}

if ($os -ne 'windows') {
    if (Get-Command chmod -ErrorAction SilentlyContinue) {
        & chmod +x $target 2>$null
    }
}

& $target @args
exit $LASTEXITCODE
