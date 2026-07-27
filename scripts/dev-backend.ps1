# dev-backend.ps1 — one-click Go backend build + restart with health check.
# Mirrors the mandatory AGENTS.md workflow so you never ship a stale binary.
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    Write-Host "==> go build ./..." -ForegroundColor Cyan
    go build ./... 2>&1 | ForEach-Object { Write-Host $_ }
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }

    Write-Host "==> go vet ./... (ignoring payload/agent unsafe.Pointer)" -ForegroundColor Cyan
    $prevEAP = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    go vet ./... 2>&1 | Where-Object { $_ -notmatch "internal[\\/]payload[\\/]agent" } | ForEach-Object { Write-Host $_ }
    $ErrorActionPreference = $prevEAP

    Write-Host "==> go mod tidy" -ForegroundColor Cyan
    go mod tidy 2>&1 | ForEach-Object { Write-Host $_ }

    Write-Host "==> build binary" -ForegroundColor Cyan
    go build -o forgec2-server.exe ./cmd/server 2>&1 | ForEach-Object { Write-Host $_ }
    if ($LASTEXITCODE -ne 0) { throw "binary build failed" }

    Write-Host "==> restart server" -ForegroundColor Cyan
    cmd /c "taskkill /f /im forgec2-server.exe >nul 2>&1"
    Start-Sleep -Seconds 1
    $p = Start-Process -WindowStyle Hidden -FilePath ".\forgec2-server.exe" -ArgumentList "-config config.yaml" -PassThru
    Start-Sleep -Seconds 3

    $health = Invoke-RestMethod -Uri "http://127.0.0.1:8000/health" -ErrorAction SilentlyContinue
    if ($null -eq $health -or $health.status -ne "ok") {
        Write-Warning "Server health check failed (PID $($p.Id))"
        exit 1
    }
    Write-Host "Backend restarted OK (PID $($p.Id))" -ForegroundColor Green
} finally {
    Pop-Location
}
