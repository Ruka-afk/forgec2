#!/usr/bin/env python3
"""Generate and inject Phase F OpenAPI paths. Idempotent: skips existing method+path."""
import re
from collections import OrderedDict

ROUTES = [
    ("post", "/api/lateral/execute", "Lateral Movement", "Execute lateral movement task", "lateralExecute"),
    ("post", "/api/lateral/result", "Lateral Movement", "Lateral movement task result", "lateralResult"),
    ("post", "/api/privesc/execute", "Privilege Escalation", "Execute privesc checks", "privescExecute"),
    ("post", "/api/privesc/result", "Privilege Escalation", "Privesc result", "privescResult"),
    ("post", "/api/privesc/run", "Privilege Escalation", "Run privesc suite", "privescRun"),
    ("post", "/portscan", "Scanner", "Port scan task", "portscan"),
    ("post", "/api/scan/result", "Scanner", "Scan result callback", "scanResult"),
    ("get", "/api/scan/export/{taskId}", "Scanner", "Export scan results", "scanExport"),
    ("post", "/agents/{id}/kill_date", "Agents", "Set agent kill date", "agentSetKillDate"),
    ("delete", "/agents/{id}/kill_date", "Agents", "Clear agent kill date", "agentClearKillDate"),
    ("get", "/settings/db/backups", "Settings", "List DB backups", "dbBackupsList"),
    ("get", "/settings/db/backups/download", "Settings", "Download DB backup", "dbBackupsDownload"),
    ("post", "/settings/db/backup", "Settings", "Create DB backup", "dbBackupCreate"),
    ("get", "/settings/totp/backup-codes/count", "Settings", "Backup codes remaining count", "totpBackupCount"),
    ("post", "/admin/emergency-stop", "Settings", "Emergency killswitch", "emergencyStop"),
    ("get", "/admin/emergency-status", "Settings", "Emergency stop status", "emergencyStatus"),
    ("post", "/socks", "SOCKS", "Start SOCKS proxy", "socksStart"),
    ("post", "/socks_relay/start", "SOCKS", "Start SOCKS relay", "socksRelayStart"),
    ("post", "/socks_relay/stop", "SOCKS", "Stop SOCKS relay", "socksRelayStop"),
    ("post", "/agents/{id}/socks_relay/start", "SOCKS", "Start agent SOCKS relay", "agentSocksRelayStart"),
    ("post", "/agents/{id}/socks_relay/stop", "SOCKS", "Stop agent SOCKS relay", "agentSocksRelayStop"),
    ("get", "/agents/{id}/socks_relay/status", "SOCKS", "Agent SOCKS relay status", "agentSocksRelayStatus"),
    ("post", "/groups", "Agents", "Create group", "groupsCreate"),
    ("put", "/groups/{id}", "Agents", "Update group", "groupsUpdate"),
    ("delete", "/groups/{id}", "Agents", "Delete group", "groupsDelete"),
    ("get", "/campaigns", "Intel", "List campaigns", "campaignsList"),
    ("post", "/campaigns", "Intel", "Create campaign", "campaignsCreate"),
    ("get", "/campaigns/{id}", "Intel", "Campaign detail", "campaignsGet"),
    ("post", "/campaigns/{id}", "Intel", "Update campaign", "campaignsUpdate"),
    ("delete", "/campaigns/{id}", "Intel", "Delete campaign", "campaignsDelete"),
    ("get", "/campaigns/{id}/mitre", "Intel", "Campaign MITRE mapping", "campaignsMitre"),
    ("post", "/campaigns/{id}/killchain", "Intel", "Campaign killchain", "campaignsKillchain"),
    ("get", "/scheduler/tasks", "Automation", "List scheduled tasks", "schedulerList"),
    ("post", "/scheduler/tasks", "Automation", "Create scheduled task", "schedulerCreate"),
    ("put", "/scheduler/tasks/{id}", "Automation", "Update scheduled task", "schedulerUpdate"),
    ("delete", "/scheduler/tasks/{id}", "Automation", "Delete scheduled task", "schedulerDelete"),
    ("post", "/scheduler/tasks/{id}/toggle", "Automation", "Toggle scheduled task", "schedulerToggle"),
    ("post", "/token/list_procs", "Tokens", "List processes for token steal", "tokenListProcs"),
    ("post", "/token/make", "Tokens", "Make token", "tokenMake"),
    ("post", "/token/steal", "Tokens", "Steal token", "tokenSteal"),
    ("post", "/token/whoami", "Tokens", "Token whoami", "tokenWhoami"),
    ("post", "/token/revert", "Tokens", "Revert token", "tokenRevert"),
    ("post", "/token/{token_id}/impersonate", "Tokens", "Impersonate token", "tokenImpersonate"),
    ("delete", "/token/{token_id}", "Tokens", "Delete stored token", "tokenDelete"),
    ("get", "/tokens", "Tokens", "List tokens", "tokensList"),
    ("post", "/screenshot", "Monitor", "Screenshot task", "screenshot"),
    ("post", "/screenshot_window", "Monitor", "Window screenshot task", "screenshotWindow"),
    ("get", "/screenshots/{agent_id}/{filename}", "Monitor", "Get screenshot file", "screenshotGet"),
    ("post", "/api/roles", "Users", "Create role", "rolesCreate"),
    ("post", "/api/roles/{id}", "Users", "Update role", "rolesUpdate"),
    ("delete", "/api/roles/{id}", "Users", "Delete role", "rolesDelete"),
    ("post", "/api/autotag/apply", "Agents", "Apply autotag rules", "autotagApply"),
    ("put", "/api/autotag/rules/{id}", "Agents", "Update autotag rule", "autotagRuleUpdate"),
    ("delete", "/api/autotag/rules/{id}", "Agents", "Delete autotag rule", "autotagRuleDelete"),
    ("post", "/api/autotag/rules/{id}/toggle", "Agents", "Toggle autotag rule", "autotagRuleToggle"),
    ("get", "/api/report/findings", "Report", "Report findings", "reportFindings"),
    ("get", "/api/report/network", "Report", "Report network section", "reportNetwork"),
    ("get", "/api/report/export/pdf", "Report", "Export report PDF", "reportExportPdf"),
    ("delete", "/api/report/{id}", "Report", "Delete report", "reportDelete"),
    ("post", "/bof", "BOF", "Upload or register BOF", "bofUpload"),
    ("get", "/bof", "BOF", "List BOFs", "bofList"),
    ("post", "/agents/{id}/bof/quick", "BOF", "Quick run BOF on agent", "bofQuick"),
]

OPENAPI = "api/openapi.yaml"


def load_existing(text: str):
    existing = set()
    path_keys = set()
    cur = None
    for line in text.splitlines():
        m = re.match(r"^  (/[^:]+):\s*$", line)
        if m:
            cur = m.group(1)
            path_keys.add(cur)
            continue
        mm = re.match(r"^    (get|post|put|delete|patch):\s*$", line, re.I)
        if mm and cur:
            existing.add(f"{mm.group(1).lower()} {cur}")
    return existing, path_keys


def method_block(method, tag, summary, opid, path):
    pnames = re.findall(r"\{([^}]+)\}", path)
    lines = [
        f"    {method}:",
        f"      tags: [{tag}]",
        f"      summary: {summary}",
        f"      operationId: {opid}",
        "      security:",
        "        - SessionCookie: []",
    ]
    if pnames:
        lines.append("      parameters:")
        for pname in pnames:
            lines.append(f"        - name: {pname}")
            lines.append("          in: path")
            lines.append("          required: true")
            lines.append("          schema: { type: string }")
    lines.append("      responses:")
    lines.append("        '200':")
    lines.append("          description: OK")
    return "\n".join(lines) + "\n"


def main():
    text = open(OPENAPI, encoding="utf-8").read()
    existing, path_keys = load_existing(text)

    by_path = OrderedDict()
    for method, path, tag, summary, opid in ROUTES:
        key = f"{method} {path}"
        if key in existing:
            continue
        if path in path_keys:
            # path already exists — skip to avoid duplicate YAML keys
            continue
        by_path.setdefault(path, []).append((method, tag, summary, opid))

    chunks = []
    added = 0
    for path, items in by_path.items():
        chunk = f"  {path}:\n"
        for method, tag, summary, opid in items:
            chunk += method_block(method, tag, summary, opid, path)
            added += 1
        chunks.append(chunk)

    if not chunks:
        print("nothing to add")
        return

    insert = "\n" + "\n".join(chunks) + "\n"
    marker = "  /api/v1/dashboard:"
    if marker not in text:
        raise SystemExit("marker /api/v1/dashboard not found")
    text = text.replace(marker, insert + marker, 1)
    open(OPENAPI, "w", encoding="utf-8", newline="\n").write(text)
    print(f"added {added} methods across {len(chunks)} paths")


if __name__ == "__main__":
    main()
