Set-Location -LiteralPath "$PSScriptRoot\.."
Set-Location frontend
Write-Host "==> npm run lint"
npm run lint
if ($LASTEXITCODE -ne 0) { exit 1 }
Write-Host "==> tsc --noEmit"
npx tsc --noEmit
if ($LASTEXITCODE -ne 0) { exit 1 }
Write-Host "==> npm run build"
npm run build
if ($LASTEXITCODE -ne 0) { exit 1 }
Write-Host "All frontend checks passed."
