const WINDOWS_COMMANDS = [
  "whoami", "hostname", "ipconfig", "ipconfig /all", "systeminfo",
  "tasklist", "tasklist /v", "netstat -ano", "netstat -anob",
  "net user", "net localgroup administrators", "net group",
  "net share", "net session", "net use",
  "reg query", "reg add", "reg delete",
  "schtasks /query", "schtasks /create",
  "wmic process list brief", "wmic service list brief",
  "wmic startup list full", "wmic product get name",
  "cmdkey /list", "cmdkey /add",
  "type", "dir", "cd", "copy", "move", "del", "mkdir",
  "more", "find", "findstr", "sort",
  "powershell", "Get-Process", "Get-Service", "Get-ChildItem",
  "Get-Content", "Get-ItemProperty", "Get-WmiObject",
  "Invoke-WebRequest", "Invoke-Expression", "Invoke-Command",
  "Start-Process", "Stop-Process", "Stop-Service",
  "Set-ExecutionPolicy", "Get-ExecutionPolicy",
  "Get-CimInstance", "Get-NetTCPConnection",
  "Export-Csv", "ConvertTo-Json", "ConvertFrom-Json",
  "Write-Host", "Write-Output", "Read-Host",
  "screenshot", "keylogger_start", "keylogger_stop", "keylogger_dump",
  "mimikatz", "creds", "kerberoast", "dcsync",
  "elevate", "uac_bypass", "amsi_bypass", "etw_bypass",
  "ps", "kill", "suspend", "resume",
  "inject", "spawn", "execute_assembly",
  "beacon_now", "set_sleep", "config_push",
  "portscan", "netstat", "drives", "users", "services", "av",
  "persistence_add", "persistence_list", "persistence_remove",
  "token_list", "token_steal", "token_make", "token_revert",
  "socks_start", "rportfwd_start", "lateral",
  "upload", "download", "find_files",
  "shell", "powershell_pickup",
  "bof", "net",
];

const LINUX_COMMANDS = [
  "whoami", "id", "uname -a", "hostname", "hostname -f",
  "ps aux", "ps -ef", "top -bn1",
  "ip addr", "ip route", "ip neigh",
  "ifconfig", "route -n", "netstat -tlnp", "ss -tlnp",
  "cat /etc/passwd", "cat /etc/shadow", "cat /etc/hosts",
  "ls -la", "ls -la /tmp", "find / -perm -4000 2>/dev/null",
  "grep -r", "grep -i", "awk", "sed", "sort", "uniq", "wc",
  "curl", "wget", "nc -e", "ncat",
  "chmod", "chown", "chgrp", "mkdir", "rmdir", "touch",
  "cp", "mv", "rm", "ln -s",
  "tar", "gzip", "gunzip", "zip", "unzip",
  "ssh", "scp", "rsync",
  "crontab -l", "crontab -e",
  "systemctl list-units", "systemctl status",
  "df -h", "du -sh", "free -m",
  "env", "printenv", "export",
  "which", "whereis", "locate",
  "screenshot", "keylogger_start", "keylogger_dump",
  "beacon_now", "set_sleep",
  "upload", "download",
];

const SUB_COMMANDS: Record<string, string[]> = {
  "netstat": ["-ano", "-anob", "-p tcp", "-p udp", "-r"],
  "ipconfig": ["/all", "/release", "/renew", "/flushdns"],
  "dir": ["/s", "/b", "/a", "/w"],
  "tasklist": ["/v", "/svc", "/fo csv", "/fo table"],
  "reg": ["query", "add", "delete", "export"],
  "wmic": ["process list brief", "service list brief", "product get name"],
  "powershell": ["-Command", "-EncodedCommand", "-File", "-NoProfile"],
  "Get-Process": ["| Select-Object", "| Format-Table", "| Where-Object"],
  "Get-Service": ["| Where-Object {$_.Status -eq 'Running'}", "| Format-Table"],
  "ssh": ["-o StrictHostKeyChecking=no", "-i", "-L", "-R", "-N"],
  "curl": ["-k", "-s", "-o", "-X GET", "-X POST", "-H", "-d", "-L"],
  "wget": ["-q", "-O", "--no-check-certificate"],
  "chmod": ["755", "777", "644", "+x"],
};

export function getCompletions(input: string, osType: string): string[] {
  const commands = osType === "linux" ? LINUX_COMMANDS : WINDOWS_COMMANDS;
  const trimmed = input.trim().toLowerCase();

  if (!trimmed) return commands.slice(0, 20);

  const parts = trimmed.split(/\s+/);
  if (parts.length >= 2) {
    const base = parts[0];
    const subCompletions = SUB_COMMANDS[base];
    if (subCompletions) {
      const subPrefix = parts.slice(1).join(" ");
      return subCompletions.filter(s => s.toLowerCase().startsWith(subPrefix));
    }
  }

  return commands.filter(c => c.toLowerCase().startsWith(trimmed));
}
