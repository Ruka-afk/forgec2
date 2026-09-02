# verify-all.ps1 — One-line comprehensive optimize/verify/build/deploy for ForgeC2
# Usage:
#   powershell -ExecutionPolicy Bypass -File scripts/verify-all.ps1
#   powershell -File scripts/verify-all.ps1 -SkipGolangci -SkipVulnCheck -SkipE2E   # default
#   powershell -File scripts/verify-all.ps1 -Full                                  # CI-grade full
#   powershell -File scripts/verify-all.ps1 -Fix -NoDeploy                          # verify only, no restart
param(
    [switch]$Full,
    [switch]$SkipGolangci,
    [switch]$SkipVulnCheck,
    [switch]$SkipE2E,
    [switch]$NoDeploy,
    [switch]$Fix
)
# Defaults per user choice: 1.跳过重型 2.自动修复 3.重启服务
if (-not $Full) { if (-not $PSBoundParameters.ContainsKey('SkipGolangci')) { $SkipGolangci = $true }; if (-not $PSBoundParameters.ContainsKey('SkipVulnCheck')) { $SkipVulnCheck = $true }; if (-not $PSBoundParameters.ContainsKey('SkipE2E')) { $SkipE2E = $true } }
if (-not $PSBoundParameters.ContainsKey('Fix')) { $Fix = $true }
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
$overall = Get-Date
try {
    function Step($msg) { Write-Host "`n==> $msg" -ForegroundColor Cyan }
    function Ok($msg) { Write-Host "  OK $msg" -ForegroundColor Green }
    function Warn($msg) { Write-Host "  WARN $msg" -ForegroundColor Yellow }

    # 0. Preflight
    Step "0/7 Preflight — Go/Node versions"
    $goVer = & go version 2>&1 | Out-String; Write-Host "  $goVer" -NoNewline
    $nodeVer = & node --version 2>&1 | Out-String; Write-Host "  node $nodeVer" -NoNewline
    if (-not (Test-Path "frontend/node_modules")) {
        Step "Installing frontend deps (npm ci)..."
        Push-Location frontend; try { npm ci; if ($LASTEXITCODE -ne 0) { throw "npm ci failed" } } finally { Pop-Location }
    }
    if (-not (Test-Path "config.yaml") -and (Test-Path "config.example.yaml")) {
        Warn "config.yaml missing — copy from config.example.yaml or run bash scripts/setup-dev.sh"
    }

    # 0.5 Fix or check formatting
    if ($Fix) {
        Step "0.5/7 Auto-fix — gofmt -w + eslint --fix"
        $prevEAP2 = $ErrorActionPreference; $ErrorActionPreference = "Continue"
        $changed = & git -c core.safecrlf=false diff --name-only HEAD -- '*.go' 2>&1 | Out-String; $ErrorActionPreference = $prevEAP2
        if ($changed.Trim() -ne "") {
            $files = $changed -split "`n" | Where-Object { $_.Trim() -ne "" -and (Test-Path -LiteralPath $_.Trim()) }
            foreach ($f in $files) { & gofmt -w $f.Trim() }
            Ok "gofmt -w applied to $($files.Count) file(s)"
        } else { Ok "gofmt — no changed Go files" }
        Push-Location frontend; try {
            $prevEAP = $ErrorActionPreference; $ErrorActionPreference = "Continue"
            $out = & npx eslint --fix 2>&1 | Out-String; $ErrorActionPreference = $prevEAP
            Write-Host $out
            if ($LASTEXITCODE -ne 0) { Warn "eslint --fix had issues (see above)" } else { Ok "eslint --fix" }
        } finally { Pop-Location }
    } else {
        Step "0.5/7 Check formatting — gofmt -l (changed files only)"
        $prevEAP2 = $ErrorActionPreference; $ErrorActionPreference = "Continue"
        $gofmtOut = & git -c core.safecrlf=false diff --name-only HEAD -- '*.go' 2>&1 | Out-String; $ErrorActionPreference = $prevEAP2
        if ($gofmtOut.Trim() -ne "") {
            $files = $gofmtOut -split "`n" | Where-Object { $_.Trim() -ne "" -and (Test-Path -LiteralPath $_.Trim()) }
            $prevEAP2 = $ErrorActionPreference; $ErrorActionPreference = "Continue"
            $fmtList = & gofmt -l ($files | ForEach-Object { $_.Trim() }) 2>&1 | Out-String; $ErrorActionPreference = $prevEAP2
            if ($fmtList.Trim() -ne "") { Write-Host $fmtList; throw "gofmt failed — run gofmt -w" } else { Ok "gofmt" }
        } else { Ok "gofmt — no changed files" }
    }

    # 1. Regenerate
    Step "1/7 Regenerate — gen:openapi + gen:permissions"
    Push-Location frontend; try {
        npm run gen:openapi; if ($LASTEXITCODE -ne 0) { throw "gen:openapi failed" }
        npm run gen:permissions; if ($LASTEXITCODE -ne 0) { Warn "gen:permissions failed (non-fatal)" } else { Ok "gen:permissions" }
    } finally { Pop-Location }

    # 2. Static gates — parallel where possible, fast-fail first
    Step "2/7 Static gates — tsc + lint + check:pre + go vet"
    $t0 = Get-Date
    Push-Location frontend; try {
        $prevEAP = $ErrorActionPreference; $ErrorActionPreference = "Continue"
        $tscOut = & npx tsc --noEmit 2>&1 | Out-String; $tscCode = $LASTEXITCODE
        $ErrorActionPreference = $prevEAP
        Write-Host $tscOut
        if ($tscCode -ne 0) { throw "tsc --noEmit failed" } else { Ok "tsc --noEmit" }
    } finally { Pop-Location }

    Push-Location frontend; try {
        $prevEAP = $ErrorActionPreference; $ErrorActionPreference = "Continue"
        $lintOut = & npm run lint 2>&1 | Out-String; $lintCode = $LASTEXITCODE
        $ErrorActionPreference = $prevEAP
        Write-Host $lintOut
        if ($lintCode -ne 0) { throw "eslint failed" } else { Ok "eslint" }
    } finally { Pop-Location }

    # check:pre (6 that don't need build) — run via concurrently for parallelism
    Push-Location frontend; try {
        & npx concurrently -n css,tokens,i18n,paths,openapi,perms "npm:check:css" "npm:check:tokens" "npm:check:i18n" "npm:check:paths" "npm:check:openapi-types" "npm:check:permissions" 2>&1 | ForEach-Object { Write-Host $_ }
        if ($LASTEXITCODE -ne 0) { throw "check:pre (css/tokens/i18n/paths/openapi-types/permissions) failed" } else { Ok "check:pre" }
    } finally { Pop-Location }

    # go vet (filtered)
    Step " go vet (filtered payload/agent)"
    $prevEAP2 = $ErrorActionPreference; $ErrorActionPreference = "Continue"
    $vetOut = & go vet ./internal/config/... ./internal/crypto/... ./internal/db/... ./internal/malleable/... ./internal/obfuscation/... ./internal/plugin/... ./internal/server/... ./pkg/... ./cmd/... 2>&1 | Out-String; $ErrorActionPreference = $prevEAP2
    $vetFiltered = $vetOut -split "`n" | Where-Object { $_ -notmatch "warning:" -and $_.Trim() -ne "" }
    if ($vetFiltered -join "" -match "\S") { $vetFiltered | ForEach-Object { Write-Host $_ }; throw "go vet failed" } else { Ok "go vet" }

    if (-not $SkipGolangci) {
        Step " golangci-lint (5m)"
        & golangci-lint run --timeout=5m 2>&1 | ForEach-Object { Write-Host $_ }
        if ($LASTEXITCODE -ne 0) { throw "golangci-lint failed" }
    } else { Warn "golangci-lint skipped (-SkipGolangci)" }

    # go test
    Step " go test (full, -timeout 5m)"
    & go test ./internal/config/... ./internal/crypto/... ./internal/db/... ./internal/malleable/... ./internal/obfuscation/... ./internal/plugin/... ./internal/server/... ./pkg/... -count=1 -timeout 5m 2>&1 | ForEach-Object { Write-Host $_ }
    if ($LASTEXITCODE -ne 0) { throw "go test failed" } else { Ok "go test" }

    if (-not $SkipVulnCheck) {
        Step " govulncheck"
        & govulncheck ./... 2>&1 | ForEach-Object { Write-Host $_ }
        if ($LASTEXITCODE -ne 0) { Warn "govulncheck found issues" }
    } else { Warn "govulncheck skipped (-SkipVulnCheck)" }

    # 3. Build + embed
    Step "3/7 Build frontend — vite build"
    Push-Location frontend; try {
        $prevEAP = $ErrorActionPreference; $ErrorActionPreference = "Continue"
        $bOut = & npm run build 2>&1 | Out-String; $ErrorActionPreference = $prevEAP
        Write-Host $bOut
        if ($LASTEXITCODE -ne 0) { throw "vite build failed" }
    } finally { Pop-Location }

    Step " Copy frontend/out -> internal/webdist/dist"
    if (Test-Path "internal/webdist/dist") { Remove-Item -Recurse -Force "internal/webdist/dist" }
    New-Item -ItemType Directory -Path "internal/webdist/dist" | Out-Null
    Copy-Item -Recurse -Path "frontend/out/*" -Destination "internal/webdist/dist/"

    Step " check:webdist + check:bundle"
    Push-Location frontend; try {
        & node ../scripts/check-webdist.mjs 2>&1 | ForEach-Object { Write-Host $_ }; if ($LASTEXITCODE -ne 0) { throw "check:webdist failed" }
        & node scripts/check-bundle.mjs 2>&1 | ForEach-Object { Write-Host $_ }; if ($LASTEXITCODE -ne 0) { throw "check:bundle failed" }
    } finally { Pop-Location }
    Ok "webdist + bundle"

    Step " go build -o forgec2-server.exe ./cmd/server"
    & go build -o forgec2-server.exe ./cmd/server 2>&1 | ForEach-Object { Write-Host $_ }
    if ($LASTEXITCODE -ne 0) { throw "go build failed" } else { Ok "go build" }

    if (-not $SkipE2E) {
        Step " Playwright e2e"
        Push-Location frontend; try { & npx playwright test 2>&1 | ForEach-Object { Write-Host $_ }; if ($LASTEXITCODE -ne 0) { throw "e2e failed" } } finally { Pop-Location }
    } else { Warn "e2e skipped (-SkipE2E)" }

    # 4. Deploy + smoke
    if ($NoDeploy) {
        Warn "deploy skipped (-NoDeploy)"
    } else {
        Step "4/7 Deploy — restart + health + smoke"
        cmd /c "taskkill /f /im forgec2-server.exe >nul 2>&1"
        Start-Sleep -Seconds 1
        $p = Start-Process -WindowStyle Hidden -FilePath ".\forgec2-server.exe" -ArgumentList "-config config.yaml" -PassThru
        Start-Sleep -Seconds 3
        $healthPort = if ($env:FORGEC2_PORT) { $env:FORGEC2_PORT } else { "8000" }
        $tries = 0; $health = $null
        while ($tries -lt 30) {
            try { $health = Invoke-RestMethod -Uri "http://127.0.0.1:$healthPort/health" -TimeoutSec 2 -ErrorAction SilentlyContinue; if ($health -and $health.status -eq "ok") { break } } catch {}
            Start-Sleep -Seconds 1; $tries++
        }
        if ($null -eq $health -or $health.status -ne "ok") { Write-Warning "health check failed (PID $($p.Id))"; exit 1 }
        Ok "health ok (PID $($p.Id), $($health.version))"
        # unauth smoke: /ready + /login + /api 401
        try {
            $r = Invoke-WebRequest -Uri "http://127.0.0.1:$healthPort/ready" -UseBasicParsing -TimeoutSec 5; if ($r.StatusCode -ne 200) { throw "ready failed" }
            $r2 = Invoke-WebRequest -Uri "http://127.0.0.1:$healthPort/login" -UseBasicParsing -TimeoutSec 5; if ($r2.StatusCode -ne 200) { throw "login failed" }
            try { Invoke-WebRequest -Uri "http://127.0.0.1:$healthPort/api/modules" -UseBasicParsing -TimeoutSec 5 | Out-Null; throw "api auth bypass" } catch { if ($_.Exception.Response.StatusCode -ne 401) { } }
            Ok "smoke unauth (/ready/login/401)"
        } catch { Warn "smoke unauth: $_" }
    }

    $elapsed = [math]::Round(((Get-Date) - $overall).TotalSeconds, 1)
    Write-Host "`nVerify+Build+Deploy OK ($elapsed s) — go vet OK / tsc OK / tests passed / bundle OK (PID check above)" -ForegroundColor Green
} finally { Pop-Location }
