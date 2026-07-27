#!/usr/bin/env python3
"""Phase G OpenAPI batch — high-value missing API routes."""
import re
from collections import OrderedDict

ROUTES = [
    ("post", "/agents/batch/delete", "Agents", "Batch delete agents", "agentsBatchDelete"),
    ("post", "/agents/batch/tags", "Agents", "Batch tag agents", "agentsBatchTags"),
    ("post", "/agents/bulk/task", "Agents", "Bulk task dispatch", "agentsBulkTask"),
    ("get", "/agents/bulk/results", "Agents", "Bulk task results", "agentsBulkResults"),
    ("get", "/agents/{id}/config", "Agents", "Agent config", "agentConfigGet"),
    ("post", "/agents/{id}/config", "Agents", "Update agent config", "agentConfigSet"),
    ("post", "/agents/{id}/trust", "Agents", "Trust agent", "agentTrust"),
    ("post", "/agents/{id}/link", "Agents", "Link agent", "agentLink"),
    ("post", "/agents/{id}/unlink", "Agents", "Unlink agent", "agentUnlink"),
    ("post", "/agents/{id}/chain/set", "Agents", "Set proxy chain", "agentChainSet"),
    ("post", "/agents/{id}/chain/clear", "Agents", "Clear proxy chain", "agentChainClear"),
    ("get", "/agents/{id}/traffic-profile", "Agents", "Traffic profile", "agentTrafficProfile"),
    ("post", "/agents/{id}/traffic-profile/adapt", "Agents", "Adapt traffic profile", "agentTrafficAdapt"),
    ("post", "/agents/{id}/traffic-profile/auto-adapt", "Agents", "Auto-adapt traffic profile", "agentTrafficAutoAdapt"),
    ("get", "/agents/{id}/shell", "Agents", "Shell session info", "agentShell"),
    ("get", "/agents/{id}/token", "Agents", "Agent token page data", "agentToken"),
    ("get", "/agents/{id}/recording", "Agents", "Recording status", "agentRecording"),
    ("get", "/agents/{id}/recording/replay", "Agents", "Recording replay", "agentRecordingReplay"),
    ("post", "/ai/chat", "AI", "AI chat", "aiChat"),
    ("post", "/ai/config", "AI", "AI config", "aiConfig"),
    ("post", "/ai/sessions", "AI", "Create AI session", "aiSessionsCreate"),
    ("get", "/ai/sessions/{id}/messages", "AI", "AI session messages", "aiSessionMessages"),
    ("post", "/ai/sessions/{id}/messages", "AI", "Post AI message", "aiSessionPostMessage"),
    ("delete", "/ai/sessions/{id}", "AI", "Delete AI session", "aiSessionDelete"),
    ("get", "/generate/builds", "Generate", "List generate builds", "generateBuilds"),
    ("get", "/generate/builds/{id}", "Generate", "Generate build detail", "generateBuildGet"),
    ("get", "/generate/builds/{id}/download", "Generate", "Download generate build", "generateBuildDownload"),
    ("post", "/generate/stager", "Generate", "Generate stager", "generateStager"),
    ("post", "/generate/stager_linux", "Generate", "Generate Linux stager", "generateStagerLinux"),
    ("post", "/generate/donut", "Generate", "Generate Donut payload", "generateDonut"),
    ("post", "/generate/one-liner", "Generate", "Generate one-liner", "generateOneLiner"),
    ("post", "/api/generate/profile/import", "Generate", "Import malleable profile", "generateProfileImport"),
    ("delete", "/api/generate/profile/{name}", "Generate", "Delete malleable profile", "generateProfileDelete"),
    ("get", "/api/templates", "Templates", "List templates", "apiTemplates"),
    ("get", "/api/traffic", "Monitor", "Traffic log", "apiTraffic"),
    ("get", "/api/translations", "Settings", "Translations", "apiTranslations"),
    ("get", "/api/translations/check", "Settings", "Translations check", "apiTranslationsCheck"),
    ("get", "/api/translations/stats", "Settings", "Translations stats", "apiTranslationsStats"),
    ("get", "/api/update-check", "Settings", "Update check", "apiUpdateCheck"),
    ("get", "/api/update-check/version", "Settings", "Update version info", "apiUpdateVersion"),
    ("post", "/api/update-check/refresh", "Settings", "Refresh update check", "apiUpdateRefresh"),
    ("post", "/api/update-check/hot-update", "Settings", "Hot update", "apiHotUpdate"),
    ("get", "/redirectors", "Infrastructure", "List redirectors", "redirectorsList"),
    ("post", "/redirectors", "Infrastructure", "Create redirector", "redirectorsCreate"),
    ("put", "/redirectors/{id}", "Infrastructure", "Update redirector", "redirectorsUpdate"),
    ("delete", "/redirectors/{id}", "Infrastructure", "Delete redirector", "redirectorsDelete"),
    ("post", "/redirectors/deploy-ssh", "Infrastructure", "Deploy redirector via SSH", "redirectorsDeploySSH"),
    ("post", "/redirectors/test-ssh", "Infrastructure", "Test redirector SSH", "redirectorsTestSSH"),
    ("post", "/redirectors/generate/{type}", "Infrastructure", "Generate redirector config", "redirectorsGenerate"),
    ("get", "/settings/certs", "Settings", "List certificates", "settingsCerts"),
    ("post", "/settings/certs/regenerate", "Settings", "Regenerate certificates", "settingsCertsRegen"),
    ("post", "/settings/certs/upload", "Settings", "Upload certificate", "settingsCertsUpload"),
    ("get", "/settings/webhooks", "Settings", "Settings webhooks", "settingsWebhooks"),
    ("post", "/settings/db/restore", "Settings", "Restore DB backup", "settingsDBRestore"),
    ("post", "/settings/maintenance/purge", "Settings", "Maintenance purge", "settingsPurge"),
    ("get", "/notifications", "Monitor", "List notifications", "notificationsList"),
    ("delete", "/notifications", "Monitor", "Clear notifications", "notificationsClear"),
    ("delete", "/notifications/{id}", "Monitor", "Delete notification", "notificationsDelete"),
    ("put", "/notifications/{id}/read", "Monitor", "Mark notification read", "notificationsRead"),
    ("put", "/notifications/read-all", "Monitor", "Mark all notifications read", "notificationsReadAll"),
    ("post", "/opsec/rules", "Monitor", "Create OPSEC rule", "opsecRulesCreate"),
    ("delete", "/opsec/rules/{name}", "Monitor", "Delete OPSEC rule", "opsecRulesDelete"),
    ("post", "/bloodhound/collect", "Intel", "BloodHound collect", "bloodhoundCollect"),
    ("post", "/bloodhound/upload", "Intel", "BloodHound upload", "bloodhoundUpload"),
    ("post", "/bloodhound/result", "Intel", "BloodHound result", "bloodhoundResult"),
    ("get", "/bloodhound/{id}/download", "Intel", "BloodHound download", "bloodhoundDownload"),
    ("delete", "/bloodhound/{id}", "Intel", "Delete BloodHound data", "bloodhoundDelete"),
    ("delete", "/phishing/campaigns/{id}", "Phishing", "Delete phishing campaign", "phishingCampaignDelete"),
    ("delete", "/phishing/templates/{id}", "Phishing", "Delete phishing template", "phishingTemplateDelete"),
    ("put", "/phishing/templates/{id}", "Phishing", "Update phishing template", "phishingTemplateUpdate"),
    ("get", "/scheduled-reports", "Report", "List scheduled reports", "scheduledReportsList"),
    ("post", "/scheduled-reports", "Report", "Create scheduled report", "scheduledReportsCreate"),
    ("put", "/scheduled-reports/{id}", "Report", "Update scheduled report", "scheduledReportsUpdate"),
    ("delete", "/scheduled-reports/{id}", "Report", "Delete scheduled report", "scheduledReportsDelete"),
    ("post", "/scheduled-reports/{id}/toggle", "Report", "Toggle scheduled report", "scheduledReportsToggle"),
    ("delete", "/api/tags/{id}", "Agents", "Delete tag", "tagsDelete"),
    ("post", "/api/scripts/execute", "Automation", "Execute script", "scriptsExecute"),
    ("post", "/api/bof/repos/import", "BOF", "Import BOF repo", "bofReposImport"),
    ("post", "/api/bof/repos/{id}/rate", "BOF", "Rate BOF repo", "bofReposRate"),
    ("get", "/users/{id}/sessions", "Users", "User sessions", "userSessions"),
    ("get", "/workflows/{id}/executions/{executionId}", "Automation", "Workflow execution detail", "workflowExecutionGet"),
    ("get", "/task-types", "Tasks", "Known task types", "taskTypes"),
    ("get", "/tasks/{id}", "Tasks", "Task detail", "taskGet"),
    ("get", "/mitre/phases", "Intel", "MITRE phases", "mitrePhases"),
    ("get", "/mitre/templates", "Intel", "MITRE templates", "mitreTemplates"),
    ("get", "/mitre/timeline", "Intel", "MITRE timeline", "mitreTimeline"),
    ("get", "/rportfwd/status", "SOCKS", "rportfwd status", "rportfwdStatus"),
    ("get", "/collab/agents", "Collaboration", "Collab agent locks", "collabAgents"),
    ("post", "/api/agents/{id}/input", "Monitor", "Remote desktop input", "agentRemoteInput"),
    ("post", "/api/agents/{id}/profile-rotate", "Agents", "Rotate malleable profile", "agentProfileRotate"),
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
