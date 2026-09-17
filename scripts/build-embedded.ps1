# build-embedded.ps1 — Build frontend, embed into Go binary, and restart server.
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    # 0. Install frontend deps when missing (keeps CI/fresh clones reproducible)
    if (-not (Test-Path "frontend/node_modules")) {
        Write-Host "==> Installing frontend dependencies..." -ForegroundColor Cyan
        Push-Location frontend
        try {
            npm ci
            if ($LASTEXITCODE -ne 0) { throw "npm ci failed" }
        } finally {
            Pop-Location
        }
    }

    # 0.5. Regenerate OpenAPI types before building. The full consistency gate
    # runs after dist has been refreshed; check:webdist would otherwise reject
    # every legitimate frontend change before this script had a chance to copy it.
    Write-Host "==> Regenerating OpenAPI types..." -ForegroundColor Cyan
    Push-Location frontend
    try {
        npm run gen:openapi
        if ($LASTEXITCODE -ne 0) { throw "openapi regeneration failed" }
    } finally {
        Pop-Location
    }

    # 0.6. Refresh the Win7 shim mirror (embedded parent sources for the
    # legacy toolchain) so the binary always embeds current sources.
    Write-Host "==> Syncing Win7 shim mirror..." -ForegroundColor Cyan
    node scripts/sync-win7shim.mjs
    if ($LASTEXITCODE -ne 0) { throw "win7shim sync failed" }

    # 1. Build frontend
    Write-Host "==> Building frontend..." -ForegroundColor Cyan
    Push-Location frontend
    # PS 5.1: native stderr lines become ErrorRecords under 2>&1 and would
    # abort the pipeline with $ErrorActionPreference=Stop. Relax locally.
    $prevEAP = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $buildOut = & npm run build 2>&1 | Out-String
    $ErrorActionPreference = $prevEAP
    Write-Host $buildOut
    if ($LASTEXITCODE -ne 0) { throw "Frontend build failed" }
    Pop-Location

    # 2. Copy output to webdist (skip when byte-identical to avoid 258-file churn)
    Write-Host "==> Syncing frontend output to webdist..." -ForegroundColor Cyan
    if (-not (Test-Path "frontend/out")) {
        throw "frontend/out missing after build — frontend build produced no output"
    }
    if (@(Get-ChildItem -Force "frontend/out").Count -eq 0) {
        throw "frontend/out is empty after build — refusing to embed an empty bundle"
    }
    $webdistFresh = $false
    if (Test-Path "internal/webdist/dist") {
        # Native stderr under $ErrorActionPreference=Stop becomes a
        # terminating ErrorRecord; relax locally like the build step above.
        $prevEAP2 = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        node scripts/check-webdist.mjs >$null 2>&1
        $checkCode = $LASTEXITCODE
        $ErrorActionPreference = $prevEAP2
        if ($checkCode -eq 0) {
            Write-Host "webdist already fresh, skipping copy" -ForegroundColor DarkGray
            $webdistFresh = $true
        }
    }
    if (-not $webdistFresh) {
        # Staged swap: a failed copy must never leave a half-removed dist
        # behind (the next go build would embed the wreckage).
        if (Test-Path "internal/webdist/dist.new") {
            Remove-Item -Recurse -Force "internal/webdist/dist.new"
        }
        New-Item -ItemType Directory -Path "internal/webdist/dist.new" | Out-Null
        Copy-Item -Recurse -Path "frontend/out/*" -Destination "internal/webdist/dist.new/"
        if (Test-Path "internal/webdist/dist") {
            Remove-Item -Recurse -Force "internal/webdist/dist"
        }
        Rename-Item -Path "internal/webdist/dist.new" -NewName "dist"
    }

    # 2.5. Validate source contracts, the freshly-built bundle and the embedded
    # copy together before compiling or restarting the backend.
    Write-Host "==> Checking frontend consistency..." -ForegroundColor Cyan
    Push-Location frontend
    try {
        npm run check
        if ($LASTEXITCODE -ne 0) { throw "frontend consistency check failed" }
    } finally {
        Pop-Location
    }

    # 3. Build backend
    Write-Host "==> Building Go backend..." -ForegroundColor Cyan
    go build -o forgec2-server.exe ./cmd/server 2>&1 | ForEach-Object { Write-Host $_ }
    if ($LASTEXITCODE -ne 0) { throw "Backend build failed" }

    # 4. Restart server
    Write-Host "==> Restarting server..." -ForegroundColor Cyan
    cmd /c "taskkill /f /im forgec2-server.exe >nul 2>&1"
    Start-Sleep -Seconds 1
    $p = Start-Process -WindowStyle Hidden -FilePath ".\forgec2-server.exe" -ArgumentList "-config config.yaml" -PassThru

    # 5. Health check (port is overridable via $env:FORGEC2_PORT to match server.port).
    # Retried like CI (30x1s): the server needs time for migrations + listener
    # bring-up. Plain HTTP first, then self-signed HTTPS (default secure
    # profile) — PS 5.1 has no -SkipCertificateCheck, so relax validation via
    # ServicePointManager for the probe only.
    $healthPort = if ($env:FORGEC2_PORT) { $env:FORGEC2_PORT } else { "8000" }
    $health = $null
    for ($i = 0; $i -lt 30; $i++) {
        Start-Sleep -Seconds 1
        if ($p.HasExited) { break }
        try {
            $health = Invoke-RestMethod -Uri "http://127.0.0.1:$healthPort/health" -TimeoutSec 5 -ErrorAction Stop
        } catch {
            $health = $null
        }
        if ($null -ne $health -and $health.status -eq "ok") { break }
        try {
            add-type @"
using System.Net;
using System.Security.Cryptography.X509Certificates;
public class TrustAllCertsPolicy : ICertificatePolicy {
    public bool CheckValidationResult(ServicePoint srvPoint, X509Certificate certificate, WebRequest request, int certificateProblem) {
        return true;
    }
}
"@ -ErrorAction SilentlyContinue
            [System.Net.ServicePointManager]::CertificatePolicy = New-Object TrustAllCertsPolicy
            $health = Invoke-RestMethod -Uri "https://127.0.0.1:$healthPort/health" -TimeoutSec 5 -ErrorAction Stop
        } catch {
            $health = $null
        }
        if ($null -ne $health -and $health.status -eq "ok") { break }
        $health = $null
    }
    if ($null -eq $health -or $health.status -ne "ok") {
        Write-Warning "Server health check failed (PID $($p.Id))"
        if (-not $p.HasExited) {
            Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
        }
        if (Test-Path "logs/forgec2.log") {
            Write-Host "--- logs/forgec2.log (tail) ---" -ForegroundColor Yellow
            Get-Content "logs/forgec2.log" -Tail 30
        }
        exit 1
    }
    Write-Host "Build + deploy complete (PID $($p.Id))" -ForegroundColor Green
} finally {
    Pop-Location
}
