# build-embedded.ps1 — Build frontend, embed into Go binary, and restart server.
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    # 1. Build frontend
    Write-Host "==> Building Next.js frontend..." -ForegroundColor Cyan
    Push-Location frontend
    npm run build 2>&1 | ForEach-Object { Write-Host $_ }
    if ($LASTEXITCODE -ne 0) { throw "Frontend build failed" }
    Pop-Location

    # 2. Copy output to webdist
    Write-Host "==> Copying frontend output to webdist..." -ForegroundColor Cyan
    if (Test-Path "internal/webdist/dist") {
        Remove-Item -Recurse -Force "internal/webdist/dist"
    }
    New-Item -ItemType Directory -Path "internal/webdist/dist" | Out-Null
    Copy-Item -Recurse -Path "frontend/out/*" -Destination "internal/webdist/dist/"

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
