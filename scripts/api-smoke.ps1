# ForgeC2 API smoke — unauthenticated + optional authenticated session checks
# Usage:
#   powershell -File scripts/api-smoke.ps1
#   powershell -File scripts/api-smoke.ps1 -User admin -Password admin
#   $env:FORGEC2_SMOKE_USER='admin'; $env:FORGEC2_SMOKE_PASS='admin'; powershell -File scripts/api-smoke.ps1
param(
    [string]$BaseUrl = "http://127.0.0.1:8000",
    [string]$User = $(if ($env:FORGEC2_SMOKE_USER) { $env:FORGEC2_SMOKE_USER } else { "" }),
    [string]$Password = $(if ($env:FORGEC2_SMOKE_PASS) { $env:FORGEC2_SMOKE_PASS } else { "" }),
    [switch]$SkipLogin,
    [switch]$TryDefaultAdmin
)

$ErrorActionPreference = "Stop"
$failed = 0
$passed = 0

function Ok($name, $detail) {
    $script:passed++
    Write-Host "[PASS] $name — $detail" -ForegroundColor Green
}
function Fail($name, $detail) {
    $script:failed++
    Write-Host "[FAIL] $name — $detail" -ForegroundColor Red
}

function Get-StatusCode($err) {
    try { return [int]$err.Exception.Response.StatusCode } catch { return $null }
}

function Get-ErrorBody($err) {
    try {
        $stream = $err.Exception.Response.GetResponseStream()
        if (-not $stream) { return "" }
        $reader = New-Object System.IO.StreamReader($stream)
        return $reader.ReadToEnd()
    } catch { return "" }
}

function Get-Csrf([Microsoft.PowerShell.Commands.WebRequestSession]$session) {
    foreach ($c in $session.Cookies.GetCookies($BaseUrl)) {
        if ($c.Name -eq "forgec2_csrf") { return $c.Value }
    }
    return $null
}

Write-Host "ForgeC2 API smoke against $BaseUrl" -ForegroundColor Cyan

# --- Unauthenticated ---

try {
    $h = Invoke-RestMethod -Uri "$BaseUrl/health" -TimeoutSec 5
    if ($h.status -eq "ok" -and $h.version) { Ok "health" "status=ok version=$($h.version)" }
    elseif ($h.status -eq "ok") { Ok "health" "status=ok (no version field)" }
    else { Fail "health" "unexpected body: $($h | ConvertTo-Json -Compress)" }
} catch {
    Fail "health" $_.Exception.Message
    Write-Host "Server not reachable. Start with: .\forgec2-server.exe -config config.yaml" -ForegroundColor Yellow
    exit 1
}

try {
    $r = Invoke-WebRequest -Uri "$BaseUrl/ready" -UseBasicParsing -TimeoutSec 5
    if ($r.StatusCode -ge 200 -and $r.StatusCode -lt 500) { Ok "ready" "HTTP $($r.StatusCode)" }
    else { Fail "ready" "HTTP $($r.StatusCode)" }
} catch {
    Fail "ready" $_.Exception.Message
}

try {
    $loginPage = Invoke-WebRequest -Uri "$BaseUrl/login" -UseBasicParsing -TimeoutSec 10
    if ($loginPage.StatusCode -eq 200 -and $loginPage.Content.Length -gt 100) {
        Ok "login SPA" "HTTP 200, $($loginPage.Content.Length) bytes"
    } else {
        Fail "login SPA" "HTTP $($loginPage.StatusCode)"
    }
} catch {
    Fail "login SPA" $_.Exception.Message
}

try {
    $null = Invoke-WebRequest -Uri "$BaseUrl/api/modules" -UseBasicParsing -TimeoutSec 5
    Fail "api modules auth" "expected 401, got HTTP 200"
} catch {
    $code = Get-StatusCode $_
    $body = Get-ErrorBody $_
    if ($code -eq 401) { Ok "api modules auth" "HTTP 401 JSON" }
    else { Fail "api modules auth" "expected 401, got $code $body" }
}

try {
    $null = Invoke-WebRequest -Uri "$BaseUrl/agents" -UseBasicParsing -TimeoutSec 5 -Headers @{ Accept = "application/json" }
    Fail "agents accept-json" "expected 401, got HTTP 200"
} catch {
    $code = Get-StatusCode $_
    if ($code -eq 401 -or $code -eq 403) { Ok "agents accept-json" "HTTP $code" }
    else { Fail "agents accept-json" "expected 401/403, got $code" }
}

# --- Authenticated ---

if ($TryDefaultAdmin -and -not $User) {
    $User = "admin"
    $Password = "admin"
}

if (-not $SkipLogin -and $User -ne "" -and $Password -ne "") {
    $session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
    try {
        $null = Invoke-WebRequest -Uri "$BaseUrl/login" -WebSession $session -UseBasicParsing -TimeoutSec 10
        $csrf = Get-Csrf $session
        $headers = @{ Accept = "application/json" }
        if ($csrf) { $headers["X-CSRF-Token"] = $csrf }

        # Form login (server uses PostForm)
        try {
            $login = Invoke-WebRequest -Uri "$BaseUrl/login" -Method POST `
                -Body @{ username = $User; password = $Password } `
                -WebSession $session -Headers $headers -UseBasicParsing -TimeoutSec 15 `
                -MaximumRedirection 0 -ErrorAction SilentlyContinue
            $loginCode = $login.StatusCode
        } catch {
            $loginCode = Get-StatusCode $_
            if (-not $loginCode) { throw }
        }

        $hasSession = $false
        foreach ($c in $session.Cookies.GetCookies($BaseUrl)) {
            if ($c.Name -eq "forgec2_session" -and $c.Value) { $hasSession = $true }
        }

        if (($loginCode -ge 200 -and $loginCode -lt 400) -or $hasSession) {
            Ok "login" "HTTP $loginCode session=$hasSession"
        } else {
            Fail "login" "HTTP $loginCode session=$hasSession"
        }

        # Refresh CSRF after login (cookie rotated on GET)
        $null = Invoke-WebRequest -Uri "$BaseUrl/api/modules" -WebSession $session -UseBasicParsing -TimeoutSec 10 -Headers @{ Accept = "application/json" } -ErrorAction SilentlyContinue
        $csrf = Get-Csrf $session
        $authHeaders = @{ Accept = "application/json" }
        if ($csrf) { $authHeaders["X-CSRF-Token"] = $csrf }

        $agents = Invoke-WebRequest -Uri "$BaseUrl/agents" -WebSession $session -Headers $authHeaders -UseBasicParsing -TimeoutSec 10
        if ($agents.StatusCode -eq 200 -and ($agents.Headers["Content-Type"] -match "json")) {
            Ok "agents list" "HTTP 200 JSON"
        } elseif ($agents.StatusCode -eq 200) {
            Ok "agents list" "HTTP 200 (ct=$($agents.Headers['Content-Type']))"
        } else {
            Fail "agents list" "HTTP $($agents.StatusCode)"
        }

        $mods = Invoke-WebRequest -Uri "$BaseUrl/api/modules" -WebSession $session -Headers $authHeaders -UseBasicParsing -TimeoutSec 10
        if ($mods.StatusCode -eq 200 -and $mods.Content -match '"success"\s*:\s*true' -and $mods.Content -match '"modules"') {
            Ok "modules" "HTTP 200 JSON envelope"
        } elseif ($mods.StatusCode -eq 200) {
            Ok "modules" "HTTP 200 (loose body)"
        } else {
            Fail "modules" "HTTP $($mods.StatusCode)"
        }

        $listeners = Invoke-WebRequest -Uri "$BaseUrl/api/listeners" -WebSession $session -Headers $authHeaders -UseBasicParsing -TimeoutSec 10
        if ($listeners.StatusCode -eq 200) { Ok "listeners" "HTTP 200" }
        else { Fail "listeners" "HTTP $($listeners.StatusCode)" }

        # Read-only product surfaces (no mutations)
        $readonly = @(
            @{ Name = "dashboard api"; Url = "$BaseUrl/api/v1/dashboard" },
            @{ Name = "task-types"; Url = "$BaseUrl/task-types" },
            @{ Name = "generate profiles"; Url = "$BaseUrl/api/generate/profiles" },
            @{ Name = "attack coverage"; Url = "$BaseUrl/attack/coverage" }
        )
        foreach ($ep in $readonly) {
            try {
                $resp = Invoke-WebRequest -Uri $ep.Url -WebSession $session -Headers $authHeaders -UseBasicParsing -TimeoutSec 15
                if ($resp.StatusCode -eq 200) { Ok $ep.Name "HTTP 200" }
                else { Fail $ep.Name "HTTP $($resp.StatusCode)" }
            } catch {
                # fallback aliases for task-types
                if ($ep.Name -eq "task-types") {
                    try {
                        $resp2 = Invoke-WebRequest -Uri "$BaseUrl/api/v1/task-types" -WebSession $session -Headers $authHeaders -UseBasicParsing -TimeoutSec 15
                        if ($resp2.StatusCode -eq 200) { Ok $ep.Name "HTTP 200 (v1)" }
                        else { Fail $ep.Name $_.Exception.Message }
                    } catch { Fail $ep.Name $_.Exception.Message }
                } else {
                    Fail $ep.Name $_.Exception.Message
                }
            }
        }

        # CSRF gate: mutating request without token must fail
        try {
            $null = Invoke-WebRequest -Uri "$BaseUrl/api/modules" -Method DELETE `
                -WebSession $session -Headers @{ Accept = "application/json" } `
                -UseBasicParsing -TimeoutSec 10
            Fail "csrf gate" "expected 403 without CSRF"
        } catch {
            $code = Get-StatusCode $_
            if ($code -eq 403 -or $code -eq 404 -or $code -eq 405) {
                Ok "csrf gate" "HTTP $code (mutation blocked/invalid without CSRF)"
            } else {
                Fail "csrf gate" "expected 403/404/405, got $code"
            }
        }

    } catch {
        Fail "authenticated suite" $_.Exception.Message
    }
} else {
    Write-Host "[SKIP] authenticated checks (pass -User/-Password or -TryDefaultAdmin)" -ForegroundColor DarkYellow
}

Write-Host ""
Write-Host "Result: $passed passed, $failed failed" -ForegroundColor $(if ($failed -eq 0) { "Green" } else { "Red" })
if ($failed -gt 0) { exit 1 }
exit 0
