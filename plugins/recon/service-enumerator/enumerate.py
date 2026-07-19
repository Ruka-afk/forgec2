#!/usr/bin/env python3
"""Service Enumerator plugin — extracts running services, systemd units, Docker containers from agent task results."""

import json
import os
import re
import sys
from collections import Counter

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

# ── Service interest categories ──────────────────────────────

DATABASE_SERVICES = [
    "mysql", "mariadb", "postgresql", "postgres", "mssql", "sqlserver",
    "mongodb", "mongod", "redis", "elasticsearch", "elastic", "memcached",
    "cassandra", "couchdb", "influxdb", "neo4j",
]

REMOTE_ACCESS_SERVICES = [
    "ssh", "sshd", "rdp", "termservice", "vnc", "teamviewer", "anydesk",
    "openssh", "telnet", "remote",
]

WEB_SERVICES = [
    "apache", "httpd", "nginx", "iis", "tomcat", "caddy", "lighttpd",
    "webserver", "w3svc",
]

SECURITY_SERVICES = [
    "windefend", "windows defender", "defender", "crowdstrike", "csfalconservice",
    "symantec", "sep", "mcafee", "sophos", "kaspersky", "sentinel", "cylance",
    "carbon black", "cbdefense", "tripwire", "ossec",
]

SERVICE_STATE_RUNNING = {"running", "active", "ok", "started", "enabled"}
SERVICE_STATE_STOPPED = {"stopped", "inactive", "dead", "failed", "disabled"}

INTEREST_CATEGORIES = {
    "database": DATABASE_SERVICES,
    "remote_access": REMOTE_ACCESS_SERVICES,
    "web": WEB_SERVICES,
    "security": SECURITY_SERVICES,
}

# ── Regex patterns for service parsing ──────────────────────

# Windows: sc query output
SC_QUERY_RE = re.compile(
    r"SERVICE_NAME:\s*(\S+).*?"
    r"DISPLAY_NAME:\s*(.+?)\s*\n.*?"
    r"STATE\s*:\s*(\d+)\s+(\S+)",
    re.DOTALL | re.IGNORECASE,
)

# Windows: Get-Service output (line-based)
GET_SERVICE_RE = re.compile(
    r"(\S+)\s+(\S+)\s+(Running|Stopped)",
    re.IGNORECASE,
)

# Windows: sc qc output (for start type / binary path)
SC_QC_RE = re.compile(
    r"START_TYPE\s*:\s*\d+\s+(\S+).*?"
    r"BINARY_PATH_NAME\s*:\s*(.+?)\s*\n",
    re.DOTALL | re.IGNORECASE,
)

# Windows: Get-WmiObject / Get-CimInstance Win32_Service CSV
WMI_SERVICE_HEADER = re.compile(
    r"Name\s*,\s*DisplayName\s*,\s*State\s*,\s*StartMode\s*,\s*StartName\s*,\s*PathName",
    re.IGNORECASE,
)

# Linux: systemctl list-units --type=service
SYSTEMCTL_RE = re.compile(
    r"^(\S+\.service)\s+(\S+)\s+(\S+)\s+(\S+)\s+(.+)",
    re.MULTILINE | re.IGNORECASE,
)

# Docker: docker ps output
DOCKER_PS_RE = re.compile(
    r"^(\S+)\s+(\S+)\s+\"(.+?)\"\s+(.+?)\s+(.+)",
    re.MULTILINE,
)

# macOS: brew services list
BREW_SERVICES_RE = re.compile(
    r"^(\S+)\s+(started|stopped|error|none)\s*\*?\s*(.*)",
    re.MULTILINE | re.IGNORECASE,
)


def _classify_service(name, path=""):
    """Determine if a service is interesting and return its category."""
    name_lower = name.lower()
    path_lower = path.lower()
    combined = name_lower + " " + path_lower

    for category, keywords in INTEREST_CATEGORIES.items():
        for kw in keywords:
            if kw in combined:
                return category
    return ""


def _is_unusual_path(path, name=""):
    """Flag third-party services with unusual installation paths."""
    if not path:
        return False
    path_lower = path.lower()
    name_lower = name.lower()

    standard_windows = [
        "c:\\windows\\system32\\", "c:\\windows\\syswow64\\",
        "c:\\program files\\", "c:\\program files (x86)\\",
        "c:\\windows\\servicing\\", "c:\\windows\\installer\\",
    ]
    standard_linux = [
        "/usr/lib/", "/usr/sbin/", "/usr/bin/", "/sbin/", "/bin/",
        "/lib/", "/usr/local/lib/", "/usr/local/sbin/", "/usr/local/bin/",
        "/snap/", "/var/snap/",
    ]

    all_standard = standard_windows + standard_linux
    for s in all_standard:
        if s in path_lower:
            return False

    # Non-standard path — interesting
    return True


def _normalize_state(state_str):
    """Normalize a state string to running/stopped."""
    s = state_str.strip().lower()
    if s in SERVICE_STATE_RUNNING:
        return "running"
    if s in SERVICE_STATE_STOPPED:
        return "stopped"
    if "running" in s or "active" in s or "ok" in s:
        return "running"
    if "stopped" in s or "inactive" in s or "dead" in s or "failed" in s:
        return "stopped"
    return s or "unknown"


def _parse_windows_sc_query(text):
    """Parse 'sc query' output for services."""
    services = []
    for m in SC_QUERY_RE.finditer(text):
        name = m.group(1).strip()
        display = m.group(2).strip()
        state = _normalize_state(m.group(4))
        services.append({
            "name": name,
            "display_name": display,
            "status": state,
            "start_type": "",
            "account": "",
            "path": "",
            "interesting": _classify_service(name),
        })
    return services


def _parse_windows_get_service(text):
    """Parse 'Get-Service' output."""
    services = []
    for m in GET_SERVICE_RE.finditer(text):
        name = m.group(1).strip()
        display = m.group(2).strip()
        state = _normalize_state(m.group(3))
        services.append({
            "name": name,
            "display_name": display,
            "status": state,
            "start_type": "",
            "account": "",
            "path": "",
            "interesting": _classify_service(name),
        })
    return services


def _parse_windows_wmi(text):
    """Parse 'Get-WmiObject Win32_Service' CSV output."""
    services = []
    lines = text.strip().splitlines()
    if not lines:
        return services

    header = None
    for line in lines:
        if WMI_SERVICE_HEADER.search(line):
            header = [h.strip().lower() for h in line.split(",")]
            continue
        if header and not line.startswith("Name"):
            parts = [p.strip().strip('"') for p in line.split(",")]
            if len(parts) < 6:
                continue
            name = parts[0] if len(parts) > 0 else ""
            display = parts[1] if len(parts) > 1 else ""
            state = _normalize_state(parts[2] if len(parts) > 2 else "")
            start_mode = parts[3] if len(parts) > 3 else ""
            account = parts[4] if len(parts) > 4 else ""
            path = parts[5] if len(parts) > 5 else ""

            if name:
                cat = _classify_service(name, path)
                if not cat and _is_unusual_path(path, name):
                    cat = "unusual_path"
                services.append({
                    "name": name,
                    "display_name": display,
                    "status": state,
                    "start_type": start_mode,
                    "account": account,
                    "path": path,
                    "interesting": cat,
                })
    return services


def _parse_systemctl(text):
    """Parse 'systemctl list-units --type=service' output."""
    services = []
    for m in SYSTEMCTL_RE.finditer(text):
        unit = m.group(1)
        load = m.group(2)
        active = _normalize_state(m.group(3))
        sub = m.group(4)
        desc = m.group(5).strip()

        name = unit.replace(".service", "")
        services.append({
            "name": name,
            "display_name": desc,
            "status": active,
            "start_type": "",
            "account": "",
            "path": "",
            "interesting": _classify_service(name),
        })
    return services


def _parse_docker_ps(text):
    """Parse 'docker ps' output."""
    services = []
    for m in DOCKER_PS_RE.finditer(text):
        name = m.group(1).strip()
        image = m.group(2).strip()
        cmd = m.group(3).strip()
        state = _normalize_state(m.group(4))

        services.append({
            "name": f"docker:{name}",
            "display_name": f"{image} ({name})",
            "status": state,
            "start_type": "docker",
            "account": "",
            "path": image,
            "interesting": _classify_service(name + " " + image),
        })
    return services


def _parse_brew_services(text):
    """Parse 'brew services list' output."""
    services = []
    for m in BREW_SERVICES_RE.finditer(text):
        name = m.group(1).strip()
        state = _normalize_state(m.group(2))
        path = m.group(3).strip()

        services.append({
            "name": name,
            "display_name": name,
            "status": state,
            "start_type": "brew",
            "account": "",
            "path": path,
            "interesting": _classify_service(name, path),
        })
    return services


def _has_service_command(cmd, result):
    """Check if task relates to service enumeration."""
    combined = (cmd + " " + result).lower()
    keywords = [
        "sc query", "get-service", "win32_service", "get-ciminstance",
        "systemctl", "service", "docker ps", "brew services",
    ]
    return any(kw in combined for kw in keywords)


def _detect_os(agent):
    """Determine OS family from agent data."""
    os_str = (agent.get("os") or "").lower()
    if any(x in os_str for x in ["windows", "win"]):
        return "windows"
    if any(x in os_str for x in ["linux", "unix", "ubuntu", "debian", "centos", "rhel", "kali"]):
        return "linux"
    if any(x in os_str for x in ["darwin", "macos", "mac os"]):
        return "macos"
    return "unknown"


def main():
    data = read_stdin()
    params = data.get("params", {})
    target_agent = params.get("agent_id") or ""

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        agents = db.all_agents()

        if target_agent:
            agents = [a for a in agents if a["id"] == target_agent]

        agent_results = []
        all_services = []
        total_running = 0
        total_stopped = 0
        category_counts = Counter()

        for agent in agents:
            aid = agent["id"]
            agent_os = _detect_os(agent)
            tasks = db.tasks_for_agent(aid)

            services = []
            seen = set()

            for task in tasks:
                if task.get("status") != "completed":
                    continue
                cmd = task.get("command", "")
                result = task.get("result", "")
                if not result or not _has_service_command(cmd, result):
                    continue

                parsed = []

                if "sc query" in cmd.lower() or "service_name:" in result[:500].lower():
                    parsed = _parse_windows_sc_query(result)
                elif "get-service" in cmd.lower() or "get-service" in result[:500].lower():
                    parsed = _parse_windows_get_service(result)
                elif "wmiobject" in cmd.lower() or "get-ciminstance" in cmd.lower() or wmi_header_found(result):
                    parsed = _parse_windows_wmi(result)
                elif "systemctl" in cmd.lower() or ".service" in result[:500].lower():
                    parsed = _parse_systemctl(result)
                elif "docker ps" in cmd.lower():
                    parsed = _parse_docker_ps(result)
                elif "brew services" in cmd.lower():
                    parsed = _parse_brew_services(result)

                for svc in parsed:
                    key = (svc["name"], svc["status"])
                    if key not in seen:
                        seen.add(key)
                        # Classify with path if available
                        cat = _classify_service(svc["name"], svc.get("path", ""))
                        if not cat and _is_unusual_path(svc.get("path", ""), svc["name"]):
                            cat = "unusual_path"
                        svc["interesting"] = cat
                        services.append(svc)

            # Deduplicate by name, keeping first occurrence
            unique = {}
            for svc in services:
                name = svc["name"]
                if name not in unique:
                    unique[name] = svc
            services = list(unique.values())

            running = sum(1 for s in services if s["status"] == "running")
            stopped = sum(1 for s in services if s["status"] == "stopped")

            agent_results.append({
                "id": aid,
                "hostname": agent.get("hostname") or aid[:8],
                "os": agent_os,
                "services": services,
            })

            all_services.extend(services)
            total_running += running
            total_stopped += stopped

            for s in services:
                if s["interesting"]:
                    category_counts[s["interesting"]] += 1

        summary = {
            "total_agents": len(agent_results),
            "total_services": len(all_services),
            "running": total_running,
            "stopped": total_stopped,
            "by_category": dict(category_counts),
        }

        output = (
            f"Scanned {len(agent_results)} agents | "
            f"{len(all_services)} services | "
            f"{total_running} running | "
            f"{total_stopped} stopped"
        )

        write_result(
            True,
            output=output,
            data={
                "agents": agent_results,
                "summary": summary,
            },
        )
    finally:
        db.close()


def wmi_header_found(text):
    """Quick check for WMI CSV header line."""
    return bool(re.search(r"Name\s*,\s*DisplayName", text[:500], re.IGNORECASE))


if __name__ == "__main__":
    main()
