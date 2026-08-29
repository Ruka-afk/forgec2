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

    # 2. Copy output to webdist
    Write-Host "==> Copying frontend output to webdist..." -ForegroundColor Cyan
    if (Test-Path "internal/webdist/dist") {
        Remove-Item -Recurse -Force "internal/webdist/dist"
    }
    New-Item -ItemType Directory -Path "internal/webdist/dist" | Out-Null
    Copy-Item -Recurse -Path "frontend/out/*" -Destination "internal/webdist/dist/"

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
    Start-Sleep -Seconds 3

    # 5. Health check (port is overridable via $env:FORGEC2_PORT to match server.port)
    $healthPort = if ($env:FORGEC2_PORT) { $env:FORGEC2_PORT } else { "8000" }
    $health = Invoke-RestMethod -Uri "http://127.0.0.1:$healthPort/health" -ErrorAction SilentlyContinue
    if ($null -eq $health -or $health.status -ne "ok") {
        Write-Warning "Server health check failed (PID $($p.Id))"
        exit 1
    }
    Write-Host "Build + deploy complete (PID $($p.Id))" -ForegroundColor Green
} finally {
    Pop-Location
}
