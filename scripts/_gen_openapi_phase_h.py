#!/usr/bin/env python3
"""Phase H OpenAPI batch — prefer multi-segment routes as extracted by checkopenapi."""
import re
from collections import OrderedDict

# Paths as extracted from Go sources (group-relative segments included as-is).
ROUTES = [
    ("post", "/api/login", "Authentication", "Login (API alias)", "apiLogin"),
    ("post", "/api/roles", "Users", "Create custom role (alias)", "apiRolesCreateAlias"),
    ("get", "/api/templates", "Templates", "List command templates API", "apiTemplatesList"),
    ("get", "/api/docs/", "Settings", "Embedded API docs", "apiDocs"),
    ("post", "/api/browser/result", "Intel", "Browser steal result callback", "browserResult"),
    ("post", "/api/wifi/result", "Intel", "WiFi cred result callback", "wifiResult"),
    ("get", "/listeners/{id}", "Listeners", "Listener detail", "listenerGet"),
    ("get", "/builds/{id}/download", "Generate", "Download build artifact", "buildsDownload"),
    ("get", "/bulk/results", "Agents", "Bulk results alias", "bulkResults"),
    ("get", "/toolkit/agents/{id}/info", "Agents", "Toolkit agent info", "toolkitAgentInfo"),
    ("get", "/toolkit/agents/{id}/tasks", "Agents", "Toolkit agent tasks", "toolkitAgentTasks"),
    ("get", "/toolkit/results", "Agents", "Toolkit results", "toolkitResults"),
    ("get", "/translations/stats", "Settings", "Translation stats alias", "translationsStatsAlias"),
    ("get", "/lang/set", "Settings", "Set language", "langSetGet"),
    ("post", "/lang/set", "Settings", "Set language POST", "langSetPost"),
    ("get", "/infrastructure/profile/export", "Infrastructure", "Export infra profile", "infraProfileExport"),
    ("get", "/integrations/malleable", "Integrations", "Malleable integrations", "integrationsMalleable"),
    ("post", "/infra/front/list", "Infrastructure", "Domain front list", "infraFrontList"),
    ("post", "/infra/front/check", "Infrastructure", "Domain front check", "infraFrontCheck"),
    ("post", "/infra/front/config", "Infrastructure", "Domain front config", "infraFrontConfig"),
    ("post", "/infrastructure/acme/provision", "Infrastructure", "ACME provision", "infraAcmeProvision"),
    ("post", "/infrastructure/generate/nginx", "Infrastructure", "Generate nginx config", "infraGenNginx"),
    ("post", "/infrastructure/generate/apache", "Infrastructure", "Generate apache config", "infraGenApache"),
    ("post", "/infrastructure/generate/haproxy", "Infrastructure", "Generate haproxy config", "infraGenHaproxy"),
    ("post", "/packer/artifact", "Generate", "Packer artifact", "packerArtifact"),
    ("post", "/payload/bundle", "Generate", "Payload bundle", "payloadBundle"),
    ("post", "/mesh/route/{agentId}", "Agents", "Set mesh route", "meshRoute"),
    ("post", "/loot/bulk-delete", "Intel", "Bulk delete loot", "lootBulkDelete"),
    ("post", "/collab/agents/{id}/lock", "Collaboration", "Lock agent", "collabAgentLock"),
    ("post", "/collab/agents/{id}/unlock", "Collaboration", "Unlock agent", "collabAgentUnlock"),
    ("post", "/collab/tasks/{taskId}/claim", "Collaboration", "Claim task", "collabTaskClaim"),
    ("post", "/collab/tasks/{taskId}/release", "Collaboration", "Release task", "collabTaskRelease"),
    ("post", "/chrome/agents/{uuid}/tasks", "Agents", "Chrome agent tasks", "chromeAgentTasks"),
    ("post", "/ai/sessions", "AI", "Create AI session alias", "aiSessionsCreateAlias"),
    ("post", "/check-updates", "Settings", "Check updates", "checkUpdates"),
    ("post", "/config/reload", "Settings", "Reload config", "configReload"),
    ("get", "/payloads/{id}/{filename}", "Generate", "Download payload file", "payloadsDownload"),
    ("get", "/stage/{xorKey}", "Generate", "Stage download", "stageDownload"),
    ("get", "/rd/{id}/frame", "Monitor", "Remote desktop frame", "rdFrame"),
    ("get", "/ws/agent", "Collaboration", "Agent websocket", "wsAgent"),
    ("get", "/ws/operator", "Collaboration", "Operator websocket", "wsOperator"),
    ("get", "/ws/remote-desktop", "Monitor", "Remote desktop websocket", "wsRemoteDesktop"),
    ("get", "/extc2/ws", "Integrations", "ExtC2 websocket", "extc2Ws"),
    ("post", "/groups", "Agents", "Create group alias", "groupsCreateAlias"),
    ("post", "/lateral", "Lateral Movement", "Lateral execute alias", "lateralAlias"),
    ("delete", "/notifications", "Monitor", "Clear all notifications", "notificationsClearAll"),
    # Common agent command routes (registered under /agents/:id in Gin; checker extracts relative path)
    ("post", "/files/ls", "Files", "List directory (agent cmd)", "agentFilesLs"),
    ("post", "/files/read", "Files", "Read file (agent cmd)", "agentFilesRead"),
    ("post", "/files/upload", "Files", "Upload file (agent cmd)", "agentFilesUpload"),
    ("post", "/files/delete", "Files", "Delete file (agent cmd)", "agentFilesDelete"),
    ("post", "/download", "Files", "Download file (agent cmd)", "agentDownload"),
    ("post", "/download_url", "Files", "Download URL (agent cmd)", "agentDownloadURL"),
    ("post", "/find", "Files", "Find files (agent cmd)", "agentFind"),
    ("post", "/drives", "Files", "List drives (agent cmd)", "agentDrives"),
    ("post", "/command", "Tasks", "Run command (agent cmd)", "agentCommand"),
    ("post", "/beacon", "Tasks", "Force beacon (agent cmd)", "agentBeacon"),
    ("post", "/beacon_now", "Tasks", "Beacon now (agent cmd)", "agentBeaconNow"),
    ("post", "/mimikatz", "Credentials", "Mimikatz (agent cmd)", "agentMimikatz"),
    ("post", "/creds", "Credentials", "Creds harvest (agent cmd)", "agentCreds"),
    ("post", "/kerberoast", "Credentials", "Kerberoast (agent cmd)", "agentKerberoast"),
    ("post", "/browser_steal", "Credentials", "Browser steal (agent cmd)", "agentBrowserSteal"),
    ("post", "/cookie_export", "Credentials", "Cookie export (agent cmd)", "agentCookieExport"),
    ("post", "/keylogger/start", "Monitor", "Keylogger start (agent cmd)", "agentKeylogStart"),
    ("post", "/keylogger/stop", "Monitor", "Keylogger stop (agent cmd)", "agentKeylogStop"),
    ("post", "/keylogger/dump", "Monitor", "Keylogger dump (agent cmd)", "agentKeylogDump"),
    ("post", "/clipboard/get", "Monitor", "Clipboard get (agent cmd)", "agentClipboardGet"),
    ("post", "/clipboard/set", "Monitor", "Clipboard set (agent cmd)", "agentClipboardSet"),
    ("post", "/net", "Agents", "Network info (agent cmd)", "agentNet"),
    ("post", "/netstat", "Agents", "Netstat (agent cmd)", "agentNetstat"),
    ("post", "/killproc", "Agents", "Kill process (agent cmd)", "agentKillproc"),
    ("post", "/inject", "Agents", "Inject (agent cmd)", "agentInject"),
    ("post", "/execute_assembly", "Agents", "Execute assembly (agent cmd)", "agentExecAssembly"),
    ("post", "/powerpick", "Agents", "Powerpick (agent cmd)", "agentPowerpick"),
    ("post", "/persistence", "Agents", "Persistence (agent cmd)", "agentPersistence"),
    ("post", "/elevate", "Privilege Escalation", "Elevate (agent cmd)", "agentElevate"),
    ("post", "/elevate/printnightmare", "Privilege Escalation", "PrintNightmare (agent cmd)", "agentPrintNightmare"),
    ("post", "/amsi_bypass", "Agents", "AMSI bypass (agent cmd)", "agentAmsiBypass"),
    ("post", "/etw_bypass", "Agents", "ETW bypass (agent cmd)", "agentEtwBypass"),
    ("post", "/amsi_hardware_bp", "Agents", "AMSI hardware BP (agent cmd)", "agentAmsiHWBP"),
    ("post", "/etw_hardware_bp", "Agents", "ETW hardware BP (agent cmd)", "agentEtwHWBP"),
    ("post", "/av", "Agents", "AV info (agent cmd)", "agentAV"),
    ("post", "/kill_av", "Agents", "Kill AV (agent cmd)", "agentKillAV"),
    ("post", "/container_detect", "Agents", "Container detect (agent cmd)", "agentContainerDetect"),
    ("post", "/container_escape", "Agents", "Container escape (agent cmd)", "agentContainerEscape"),
    ("post", "/container_docker", "Agents", "Container docker (agent cmd)", "agentContainerDocker"),
    ("post", "/container_k8s", "Agents", "Container k8s (agent cmd)", "agentContainerK8s"),
    ("post", "/modules/deploy", "Agents", "Modules deploy (agent cmd)", "agentModulesDeploy"),
    ("post", "/coerce/{type}", "Lateral Movement", "Coerce type (agent cmd)", "agentCoerceType"),
    ("post", "/import", "Agents", "Import (agent cmd)", "agentImport"),
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
        if key in existing or path in path_keys:
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
        raise SystemExit("marker not found")
    text = text.replace(marker, insert + marker, 1)
    open(OPENAPI, "w", encoding="utf-8", newline="\n").write(text)
    print(f"added {added} methods across {len(chunks)} paths")


if __name__ == "__main__":
    main()
