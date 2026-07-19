#!/usr/bin/env python3
import sys, os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result


def classify_integrity(integrity: str) -> str:
    val = (integrity or "").lower().strip()
    if val in ("system", "high"):
        return "high"
    if val in ("medium",):
        return "medium"
    if val in ("low",):
        return "low"
    return "unknown"


def is_system_user(username: str) -> bool:
    u = (username or "").lower().strip()
    return u in ("nt authority\\system", "system", "root", "nt authority\\local service", "nt authority\\network service")


def is_admin_user(username: str) -> bool:
    u = (username or "").lower().strip()
    if u.endswith("\\administrator"):
        return True
    if u == "administrator":
        return True
    if u == "root":
        return True
    return False


def is_domain_joined(domain: str) -> bool:
    d = (domain or "").strip()
    if not d:
        return False
    return d.lower() not in ("", "workgroup", "unknown", "n/a", "-")


def get_escalation_opportunities(agent: dict) -> list:
    opportunities = []
    integrity = classify_integrity(agent.get("integrity", ""))
    elevated = agent.get("elevated", False)
    os_type = (agent.get("os") or "").lower()
    username = agent.get("username") or ""
    domain = agent.get("domain") or ""

    if "windows" in os_type:
        if integrity == "low":
            opportunities.append({
                "technique": "UAC Bypass",
                "reason": "Agent running at Low integrity level",
                "severity": "high",
                "suggestions": [
                    " fodexploit, EventViewer, sdclt, ComputerDefaults bypasses",
                    "CMSTP / MSHTA / .NET Assembly loading",
                ],
            })
        if integrity == "medium":
            opportunities.append({
                "technique": "UAC Bypass",
                "reason": "Agent running at Medium integrity (admin but not elevated)",
                "severity": "high",
                "suggestions": [
                    "Fodhelper / ComputerDefaults / EventViewer bypass",
                    "CMSTP / MSHTA auto-elevation",
                    "DLL hijack in writable system directories",
                ],
            })
        if elevated and integrity in ("medium", "high") and not is_system_user(username):
            opportunities.append({
                "technique": "Token Impersonation",
                "reason": "Agent running as admin but not SYSTEM",
                "severity": "medium",
                "suggestions": [
                    "Check for impersonatable tokens (SeImpersonatePrivilege)",
                    "Use Incognito to list available tokens",
                    "Potato attacks if SeImpersonatePrivilege is present",
                ],
            })
        if is_domain_joined(domain):
            opportunities.append({
                "technique": "Kerberoasting / AS-REP Roasting",
                "reason": f"Agent is domain-joined ({domain})",
                "severity": "medium",
                "suggestions": [
                    "Request TGS for service accounts and crack offline",
                    "Check for AS-REP-able accounts (no preauth required)",
                    "SPN scanning for targeting",
                ],
            })
        if integrity == "high" and not is_system_user(username):
            opportunities.append({
                "technique": "Direct SYSTEM Access",
                "reason": "High integrity but not SYSTEM — SeImpersonatePrivilege likely available",
                "severity": "low",
                "suggestions": [
                    "Potato family attacks for SYSTEM escalation",
                    "Check token privileges with whoami /priv",
                ],
            })
    elif "linux" in os_type:
        if not elevated:
            opportunities.append({
                "technique": "Sudo Misconfiguration",
                "reason": "Agent not running as root",
                "severity": "medium",
                "suggestions": [
                    "Check sudo -l for NOPASSWD entries",
                    "GTFOBins for privileged binaries",
                ],
            })
            opportunities.append({
                "technique": "Capabilities Abuse",
                "reason": "Check for binaries with elevated capabilities",
                "severity": "medium",
                "suggestions": [
                    "getcap -r / to find binaries with capabilities",
                    "exploit capability on interpreters or compilers",
                ],
            })
            opportunities.append({
                "technique": "Cron Job Hijack",
                "reason": "Check for writable cron scripts run by root",
                "severity": "medium",
                "suggestions": [
                    "List cron jobs: crontab -l && ls /etc/cron*",
                    "Check writable scripts in cron entries",
                ],
            })
        if is_domain_joined(domain):
            opportunities.append({
                "technique": "Linux AD Integration",
                "reason": f"Agent is domain-joined ({domain})",
                "severity": "medium",
                "suggestions": [
                    "Check for cached AD credentials",
                    "Enumerate AD service accounts via SPNs",
                ],
            })

    if not opportunities:
        opportunities.append({
            "technique": "No Escalation Needed",
            "reason": "Agent already running with highest available privileges",
            "severity": "info",
            "suggestions": [],
        })

    return opportunities


def main():
    data = read_stdin()
    agent_id = data.get("agent_id", "")

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        if agent_id:
            agent = db.agent_by_id(agent_id)
            agents = [agent] if agent else []
        else:
            agents = db.all_agents()

        analyzed = []
        high_priv = 0
        medium_priv = 0
        low_priv = 0
        escalation_candidates = 0

        for agent in agents:
            integrity = classify_integrity(agent.get("integrity", ""))
            escalated = agent.get("elevated", False)

            if integrity == "high":
                high_priv += 1
            elif integrity == "medium":
                medium_priv += 1
            elif integrity == "low":
                low_priv += 1

            opps = get_escalation_opportunities(agent)
            has_escalation = any(o["technique"] != "No Escalation Needed" for o in opps)
            if has_escalation:
                escalation_candidates += 1

            analyzed.append({
                "id": agent.get("id"),
                "hostname": agent.get("hostname"),
                "username": agent.get("username"),
                "os": agent.get("os"),
                "integrity": agent.get("integrity"),
                "elevated": escalated,
                "escalation_opportunities": opps,
            })

        total = len(analyzed)
        output = f"Analyzed {total} agents | {high_priv} high privilege | {escalation_candidates} escalation candidates"

        write_result(
            True,
            output=output,
            data={
                "agents": analyzed,
                "summary": {
                    "total": total,
                    "high_priv": high_priv,
                    "medium_priv": medium_priv,
                    "low_priv": low_priv,
                    "escalation_candidates": escalation_candidates,
                },
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
