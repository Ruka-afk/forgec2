# ForgeC2 C-implant local E2E: throwaway server, minted secret, real tasks.
#
#   powershell -NoProfile -File scripts\e2e-c-implant-local.ps1
#
# This is the reproducible counterpart to the CI build gate: CI proves the
# implant compiles and ships the expected task strings, this script proves the
# tasks actually work against a live server. It never touches an operator's
# server, database or %TEMP% identity:
#
#   * builds a throwaway server (fresh data dir, plain HTTP, generated
#     beacon key and admin password; nothing is written outside -WorkDir)
#   * mints a v3 registration secret with cmd/mint-reg-secret (no UI needed)
#   * cross-builds the implant with mingw-w64 against that secret
#   * runs it with TEMP/TMP redirected, so %TEMP%\fc2c.dat stays untouched
#   * drives scripts/e2e-c-implant.bat once per task and asserts the output
#
# Task matrix: shell, drives, find, clipboard_set/get, portscan,
# get_sleep_mode, usb_enum, session_recon, token_whoami, plus two usb_drop
# refusal cases (the implant must refuse to copy itself, and must refuse a
# fixed-volume destination) that hold on any machine, USB stick or not.
#
# Requires: go, x86_64-w64-mingw32-gcc, curl.exe, PowerShell 5.1.
# Note: clipboard tasks need an interactive desktop; without one they must
# fail closed and are reported as "env skip" rather than as failures.
[CmdletBinding()]
param(
    [string]$RepoRoot,
    [string]$WorkDir = (Join-Path $env:TEMP 'forgec2-c-implant-e2e'),
    [int]$Port = 8100,
    [string]$User = 'admin',
    [string]$Password,
    [string]$BeaconKey,
    [switch]$KeepWork
)

$ErrorActionPreference = 'Stop'

function Step([string]$m) { Write-Host "=== $m ===" }

function New-StrongPassword {
    $bytes = New-Object byte[] 8
    [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    $hex = -join ($bytes | ForEach-Object { '{0:x2}' -f $_ })
    # Needs upper/lower/digit and >= 12 chars (config.isWeakDefaultPassword).
    # Deliberately free of cmd metacharacters: e2e-c-implant.bat runs with
    # enabledelayedexpansion, which eats "!" and would corrupt the password.
    return "E2e-Xk9-$hex"
}

if (-not $Password) { $Password = New-StrongPassword }
if (-not $BeaconKey) {
    $bytes = New-Object byte[] 32
    [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    $BeaconKey = -join ($bytes | ForEach-Object { '{0:x2}' -f $_ })
}

$server = $null
$implant = $null
$pushed = $false
$fail = New-Object System.Collections.ArrayList

function Check([string]$name, [bool]$cond, [string]$detail) {
    if ($cond) {
        Write-Host "  PASS $name $detail"
    } else {
        Write-Host "  FAIL $name $detail"
        [void]$fail.Add($name)
    }
}

function Get-AgentList {
    # curl, not Invoke-RestMethod: its private CookieContainer silently ignores
    # a Cookie header, so a valid session would answer 401. Reading with UTF-8
    # is required too: hostnames are not always ASCII.
    & curl.exe -s -b $script:jar -H 'Accept: application/json' -o "$WorkDir/agents.json" "http://127.0.0.1:$Port/api/v1/agents"
    return ([System.IO.File]::ReadAllText("$WorkDir/agents.json", [System.Text.Encoding]::UTF8) | ConvertFrom-Json)
}

try {
    if (-not $RepoRoot) {
        # $PSScriptRoot is still empty while a param() default is evaluated, so
        # resolve the repository here instead.
        $scriptPath = $MyInvocation.MyCommand.Path
        if (-not $scriptPath) { $scriptPath = $PSCommandPath }
        $RepoRoot = Split-Path -Parent (Split-Path -Parent $scriptPath)
    }
    $WorkDir = [System.IO.Path]::GetFullPath($WorkDir)
    $RepoRoot = [System.IO.Path]::GetFullPath($RepoRoot)
    Step "work dir: $WorkDir"
    if (Test-Path $WorkDir) { Remove-Item -Recurse -Force $WorkDir }
    New-Item -ItemType Directory -Force -Path "$WorkDir/tmp" | Out-Null
    Push-Location $RepoRoot
    $pushed = $true

    Step '1/8 build the throwaway server'
    go build -o "$WorkDir/forgec2-server.exe" ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw 'server build failed' }

    Step '2/8 write the throwaway config'
    $cfg = Get-Content config.example.yaml -Raw
    $cfg = $cfg -replace '(?m)^(\s*)port: 8000', "`${1}port: $Port"
    $cfg = $cfg -replace '(?m)^(\s*)host: 0.0.0.0', "`${1}host: 127.0.0.1"
    $cfg = $cfg -replace '(?m)^(\s*)tls_enabled: true', "`${1}tls_enabled: false"
    $cfg = $cfg -replace '(?m)^(\s*)require_tls_for_auth: true', "`${1}require_tls_for_auth: false"
    # No end-of-line anchor: the example carries a trailing comment on the
    # data_dir line, and the trailing text must survive the rewrite.
    $cfg = $cfg -replace '(?m)^([ \t]*)data_dir: data\b', "`${1}data_dir: $WorkDir/data"
    $cfg = $cfg -replace '(?m)^(\s*)path: data/db/forgec2.db', "`${1}path: $WorkDir/data/db/forgec2.db"
    $cfg = $cfg -replace '(?m)^(\s*)beacon_key: ""', "`${1}beacon_key: `"$BeaconKey`""
    $cfg = $cfg -replace '(?m)^(\s*)default_password: ""', "`${1}default_password: `"$Password`""
    # The two-man rule is a server-side gate with its own tests; this script
    # drives the implant, so high-impact types (usb_drop) must reach it
    # directly instead of parking in pending_approval (the documented
    # single-operator lab setting; the server logs a startup warning).
    $cfg = $cfg -replace '(?m)^(\s*)require_approval: true', "`${1}require_approval: false"
    if ($cfg -notmatch "port: $Port" -or $cfg -notmatch 'tls_enabled: false' -or
        $cfg -notmatch 'require_tls_for_auth: false' -or $cfg -notmatch 'require_approval: false' -or
        $cfg -notmatch [regex]::Escape("data_dir: $WorkDir/data") -or
        $cfg -notmatch [regex]::Escape("path: $WorkDir/data/db/forgec2.db") -or
        $cfg -notmatch [regex]::Escape($Password) -or $cfg -notmatch [regex]::Escape($BeaconKey)) {
        throw 'config rewrite did not apply cleanly - is config.example.yaml unchanged?'
    }
    # No BOM: the server's YAML reader does not need one, and UTF-8 keeps any
    # non-ASCII comment intact.
    [System.IO.File]::WriteAllText("$WorkDir/config.e2e.yaml", $cfg, (New-Object System.Text.UTF8Encoding($false)))
    Write-Host "  wrote $WorkDir/config.e2e.yaml"

    Step "3/8 start the throwaway server on 127.0.0.1:$Port"
    $server = Start-Process -FilePath "$WorkDir/forgec2-server.exe" `
        -ArgumentList '-config', "$WorkDir/config.e2e.yaml" -WorkingDirectory $WorkDir -PassThru -WindowStyle Hidden
    $healthy = $false
    for ($i = 0; $i -lt 80; $i++) {
        Start-Sleep -Milliseconds 500
        try {
            $r = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/health" -TimeoutSec 2
            if ($r.Content -match 'ok') { $healthy = $true; break }
        } catch { }
    }
    if (-not $healthy) { throw "the throwaway server never became healthy on port $Port" }
    Write-Host '  /health ok'

    Step '4/8 log in and mint a registration secret'
    # A successful /api/login answers 302 towards the SPA; curl keeps the
    # Set-Cookie header either way (and does not follow redirects by default).
    $script:jar = "$WorkDir/cookies.txt"
    $code = & curl.exe -s -c $script:jar -o "$WorkDir/login-body.txt" -w '%{http_code}' `
        -H 'Accept: application/json' -d "username=$User&password=$Password" "http://127.0.0.1:$Port/api/login"
    if ($code -ne '302' -and $code -ne '200') {
        throw "login failed with HTTP ${code}: $(Get-Content "$WorkDir/login-body.txt" -Raw)"
    }
    $mint = & go run ./cmd/mint-reg-secret -db "$WorkDir/data/db/forgec2.db" -beacon-key $BeaconKey
    if ($LASTEXITCODE -ne 0) { throw 'cmd/mint-reg-secret failed' }
    $sid = (($mint | Where-Object { $_ -like 'set SECRET_ID=*' }) -replace '^set SECRET_ID=', '')
    $sb64 = (($mint | Where-Object { $_ -like 'set SECRET_B64=*' }) -replace '^set SECRET_B64=', '')
    if (-not $sid -or -not $sb64) { throw "could not parse mint output: $mint" }
    Write-Host "  minted secret $sid"

    Step '5/8 cross-build the implant against that secret'
    # sqlite3.c is a 9MB vendored amalgamation; compiling it once and caching
    # the object keeps repeated E2E loops fast.
    if (-not (Test-Path "$WorkDir/sqlite3.o") -or
        (Get-Item "$WorkDir/sqlite3.o").LastWriteTime -lt (Get-Item "$RepoRoot\proto\c-implant\sqlite3.c").LastWriteTime) {
        Write-Host '  compiling sqlite3.c (first run, ~40s)...'
        cmd /c "cd /d `"$RepoRoot\proto\c-implant`" && x86_64-w64-mingw32-gcc -O1 -c sqlite3.c -o `"$WorkDir/sqlite3.o`" > `"$WorkDir/sqlite3.log`" 2>&1"
        if (-not (Test-Path "$WorkDir/sqlite3.o")) { Get-Content "$WorkDir/sqlite3.log" -Tail 15 | ForEach-Object { Write-Host "  $_" }; throw 'sqlite3.o build failed' }
    }
    # Generated as a .cmd: PowerShell mangles the -Wl/-D quoting that mingw needs.
    $build = @"
@echo off
cd /d $RepoRoot\proto\c-implant
x86_64-w64-mingw32-gcc -O2 -Wall -o "$WorkDir/cbeacon-e2e.exe" beacon.c crypto_cng.c curve25519.c evade.c dns_transport.c "$WorkDir/sqlite3.o" -lwinhttp -lbcrypt -lpsapi -liphlpapi -lgdiplus -lgdi32 -luser32 -lole32 -ldnsapi -lws2_32 -DC2_HOST=\"127.0.0.1\" -DC2_PORT=$Port -DBEACON_PATH=\"/collect\" -DSECRET_ID=\"$sid\" -DSECRET_B64=\"$sb64\" -DINTERVAL=3 > "$WorkDir/build.log" 2>&1
"@
    [System.IO.File]::WriteAllText("$WorkDir/build-implant.cmd", $build, (New-Object System.Text.UTF8Encoding($false)))
    cmd /c "$WorkDir/build-implant.cmd"
    if (-not (Test-Path "$WorkDir/cbeacon-e2e.exe")) {
        Get-Content "$WorkDir/build.log" -Tail 15 | ForEach-Object { Write-Host "  $_" }
        throw 'implant build failed'
    }
    Write-Host "  built $((Get-Item "$WorkDir/cbeacon-e2e.exe").Length) bytes"

    Step '6/8 run the implant with a scratch TEMP and wait for registration'
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = "$WorkDir/cbeacon-e2e.exe"
    $psi.WorkingDirectory = $WorkDir
    $psi.UseShellExecute = $false
    $psi.EnvironmentVariables['TEMP'] = "$WorkDir/tmp"
    $psi.EnvironmentVariables['TMP'] = "$WorkDir/tmp"
    $implant = [System.Diagnostics.Process]::Start($psi)
    $agentId = $null
    for ($i = 0; $i -lt 60; $i++) {
        Start-Sleep -Seconds 1
        try { $agents = @((Get-AgentList).data) } catch { continue }
        if ($agents.Count -gt 0) { $agentId = $agents[0].id; $status = $agents[0].status; break }
    }
    if (-not $agentId) { throw 'the implant never registered' }
    Write-Host "  agent $agentId status=$status"
    if ($status -ne 'online') { throw "agent is not online: $status" }

    Step '7/8 run the task matrix through scripts/e2e-c-implant.bat'
    $extraJson = '"path":"' + ($WorkDir -replace '\\', '\\') + '"'
    # usb_drop takes the source as path and the destination as command; both
    # extras below point at files that exist. `C:` as a destination is a valid
    # JSON command with no backslash to escape, and is never removable.
    $selfExtra = '"path":"' + (($WorkDir + '\cbeacon-e2e.exe') -replace '\\', '\\') + '"'
    $logExtra = '"path":"' + (($WorkDir + '\build.log') -replace '\\', '\\') + '"'
    $cases = @(
        @{ name = 'shell';          type = 'shell';          cmd = 'echo CBEACON-E2E-VERIFIED'; extra = '' },
        @{ name = 'drives';         type = 'drives';         cmd = '';                          extra = '' },
        @{ name = 'find';           type = 'find';           cmd = '*.exe';                     extra = $extraJson },
        @{ name = 'clipboard_set';  type = 'clipboard_set';  cmd = 'CBEACON-CLIP-MARKER';       extra = '' },
        @{ name = 'clipboard_get';  type = 'clipboard_get';  cmd = '';                          extra = '' },
        @{ name = 'portscan';       type = 'portscan';       cmd = "127.0.0.1:$Port";           extra = '' },
        @{ name = 'get_sleep_mode'; type = 'get_sleep_mode'; cmd = '';                          extra = '' },
        @{ name = 'usb_enum';       type = 'usb_enum';       cmd = '';                          extra = '' },
        @{ name = 'session_recon';  type = 'session_recon';  cmd = '';                          extra = '' },
        @{ name = 'token_whoami';   type = 'token_whoami';   cmd = '';                          extra = '' },
        @{ name = 'usb_drop_self';  type = 'usb_drop';       cmd = '';                          extra = $selfExtra },
        @{ name = 'usb_drop_fixed'; type = 'usb_drop';       cmd = 'C:';                        extra = $logExtra }
    )
    $outputs = @{}
    foreach ($case in $cases) {
        # Only the JSON fragment is passed through the environment: cmd strips
        # quotes and backslashes from arguments.
        if ($case.extra) { $env:E2E_EXTRA_JSON = $case.extra } else { Remove-Item Env:E2E_EXTRA_JSON -ErrorAction SilentlyContinue }
        $prevEap = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        $out = (& "$RepoRoot\scripts\e2e-c-implant.bat" 127.0.0.1 $Port $User $Password $agentId $case.type $case.cmd 2>&1 | Out-String)
        $code = $LASTEXITCODE
        $ErrorActionPreference = $prevEap
        $outputs[$case.name] = $out
        Write-Host "  [$($case.name)] exit=$code"
        if ($code -ne 0) {
            ($out -split "`r?`n" | Select-Object -Last 8) | ForEach-Object { Write-Host "      $_" }
        }
    }

    Step '8/8 assertions'
    # Completion first: a marker that also shows up in the script's own [E2E]
    # echo lines must not be able to pass on its own.
    foreach ($case in $cases) {
        if ($case.type -like 'clipboard*') {
            # No interactive desktop (CI, service session) -> the task must
            # fail closed with the documented error instead of succeeding.
            $done = $outputs[$case.name] -match 'OK: Task completed'
            $closed = $outputs[$case.name] -match 'Error: cannot open the clipboard' -or
                      $outputs[$case.name] -match 'Error: clipboard set failed'
            Check "$($case.name) (completed or failed closed)" ($done -or $closed) $(if ($done) { '' } else { '(no usable clipboard here)' })
        } elseif ($case.type -eq 'usb_drop') {
            # The two usb_drop cases are refusal tests: they must fail closed
            # with a usb_drop: reason, never "complete".
            Check "$($case.name) failed closed" ($outputs[$case.name] -match 'Error: usb_drop: ') ''
        } else {
            Check "$($case.name) completed" ($outputs[$case.name] -match 'OK: Task completed') ''
        }
    }
    Check 'shell output' ($outputs['shell'] -match 'CBEACON-E2E-VERIFIED') ''
    Check 'drives table' ($outputs['drives'] -match "Drive`tType`tFree`tTotal" -and $outputs['drives'] -match "`tfixed`t") ''
    # Base64 task output is decoded by the server before storage, so find and
    # clipboard_get results reach the operator as plain text.
    Check 'find hit' ($outputs['find'] -match "cbeacon-e2e\.exe`t\d+`t\d{4}-\d{2}-\d{2}") ''
    if ($outputs['clipboard_set'] -match 'OK: Task completed') {
        Check 'clipboard_set output' ($outputs['clipboard_set'] -match 'clipboard set') ''
    }
    if ($outputs['clipboard_get'] -match 'OK: Task completed') {
        Check 'clipboard_get round-trip' ($outputs['clipboard_get'] -match 'CBEACON-CLIP-MARKER') ''
    }
    Check 'portscan open' ($outputs['portscan'] -match "127\.0\.0\.1:$Port open") ''
    Check 'get_sleep_mode' ($outputs['get_sleep_mode'] -match 'sleep interval 3 s') ''
    Check 'usb_enum header' ($outputs['usb_enum'] -match '=== volume enum \(Windows GetDriveType\) ===' -and
                            $outputs['usb_enum'] -match "root`ttype`tlabel`tfree`ttotal") ''
    Check 'usb_enum volume row' ($outputs['usb_enum'] -match "`t[A-Z]:\\`tfixed`t") ''
    Check 'session_recon sections' ($outputs['session_recon'] -match 'console_session_id=\d+' -and
                                    $outputs['session_recon'] -match '--- whoami ---') ''
    Check 'token_whoami current user' ($outputs['token_whoami'] -match '"whoami":"' -and
                                       $outputs['token_whoami'] -match [regex]::Escape($env:USERNAME)) ''
    Check 'usb_drop refuses the implant' ($outputs['usb_drop_self'] -match 'usb_drop: refused to copy the implant executable') ''
    Check 'usb_drop refuses a fixed destination' ($outputs['usb_drop_fixed'] -match 'usb_drop: destination C: is not a removable volume') ''
}
finally {
    if ($implant -and -not $implant.HasExited) { $implant.Kill(); Write-Host '  implant stopped' }
    if ($server -and -not $server.HasExited) { $server.Kill(); Write-Host '  server stopped' }
    if ($pushed) { Pop-Location }
    if (-not $KeepWork -and $fail.Count -eq 0) {
        # Drop the throwaway DB, config, cookie jar and binaries of a green run
        # (they carry a minted secret); keep the sqlite3.o cache for next time.
        foreach ($p in @('cbeacon-e2e.exe', 'config.e2e.yaml', 'cookies.txt', 'data', 'tmp')) {
            Remove-Item -Recurse -Force (Join-Path $WorkDir $p) -ErrorAction SilentlyContinue
        }
        Write-Host '  cleaned run artifacts (sqlite3.o cache kept)'
    } else {
        Write-Host "  artifacts kept in $WorkDir"
    }
}

Write-Host ''
if ($fail.Count -gt 0) { Write-Host "C-implant E2E FAILED: $($fail -join ', ')"; exit 1 }
Write-Host 'C-implant E2E ALL GREEN'
exit 0
