# build-agent.ps1 — Cross-platform agent compilation verification.
# Verifies that the agent payload compiles for all target platforms.
# Run before deploying new agent builds or after any agent/ changes.
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    # Agent implant is Windows/amd64 only (syscall stubs, Windows APIs, unsafe.Pointer alignment).
    # Cross-compilation for other targets is expected to fail.
    # This script verifies the primary payload builds cleanly.
    $targets = @(
        @{ GOOS="windows"; GOARCH="amd64"; dir="internal/payload/agent"; out="forgec2-agent.exe" }
    )

    # Verify Windows stager too
    $stagerTarget = @{ GOOS="windows"; GOARCH="amd64"; dir="internal/payload/agent_stager"; out="forgec2-stager.exe" }

    $failures = 0
    $successes = 0

    Write-Host "=== Agent Cross-Compilation Matrix ===" -ForegroundColor Cyan
    Write-Host ""

    foreach ($t in $targets + @($stagerTarget)) {
        $env:GOOS = $t.GOOS
        $env:GOARCH = $t.GOARCH
        $env:CGO_ENABLED = "0"
        $label = "$($t.GOOS)/$($t.GOARCH) $($t.dir)"
        Write-Host "  Building $label ... " -NoNewline

        $output = go build -o "forgec2-agent-build-verify" "./$($t.dir)" 2>&1
        if ($LASTEXITCODE -eq 0) {
            if (Test-Path "forgec2-agent-build-verify") {
                $size = (Get-Item "forgec2-agent-build-verify").Length
                Remove-Item -Force "forgec2-agent-build-verify"
                Write-Host "OK ($([math]::Round($size / 1KB)) KB)" -ForegroundColor Green
                $successes++
            } else {
                Write-Host "OK (no binary? unexpected)" -ForegroundColor Yellow
                $successes++
            }
        } else {
            Write-Host "FAIL" -ForegroundColor Red
            $output | ForEach-Object { Write-Host "    $_" }
            $failures++
        }
    }

    Write-Host ""
    Write-Host "=== Results: $successes succeeded, $failures failed ===" -ForegroundColor Cyan

    if ($failures -gt 0) {
        Write-Host "Some agent builds failed — check compatibility of recent changes." -ForegroundColor Yellow
        exit 1
    }

    Write-Host "All agent targets compile successfully." -ForegroundColor Green
} finally {
    Remove-Item -ErrorAction SilentlyContinue -Force "forgec2-agent-build-verify" 2>$null
    Pop-Location
}
