param(
    [switch]$StartService
)

Write-Host "=== Step 1: go mod tidy ===" -ForegroundColor Cyan
go mod tidy
if ($LASTEXITCODE -ne 0) {
    Write-Host "FAILED: go mod tidy" -ForegroundColor Red
    exit 1
}

Write-Host "=== Step 2: go build ./... ===" -ForegroundColor Cyan
go build ./...
if ($LASTEXITCODE -ne 0) {
    Write-Host "FAILED: go build" -ForegroundColor Red
    exit 1
}

Write-Host "=== Step 3: go vet ./... (ignoring unsafe.Pointer in agent) ===" -ForegroundColor Cyan
go vet ./... 2>&1 | Where-Object { $_ -notmatch "unsafe.Pointer" }
if ($LASTEXITCODE -ne 0) {
    Write-Host "FAILED: go vet" -ForegroundColor Red
    exit 1
}

Write-Host "=== ALL CHECKS PASSED ===" -ForegroundColor Green

if ($StartService) {
    Write-Host "=== Starting server... ===" -ForegroundColor Cyan
    go run ./cmd/server -config config.yaml
}
