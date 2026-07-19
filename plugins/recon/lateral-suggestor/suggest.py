#!/usr/bin/env python3
"""Lateral Suggestor plugin — analyzes collected data and suggests lateral movement paths."""

import ipaddress
import json
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

WINDOWS_KEYWORDS = {"windows", "win32", "win64", "windows nt"}
LINUX_KEYWORDS = {"linux", "unix", "ubuntu", "debian", "centos", "rhel", "kali"}

HIGH_VALUE_USERS = {"administrator", "admin", "root", "sa", "krbtgt", "domain admin"}

LATERAL_METHODS = {
    ("windows", "windows"): ["smb", "wmi", "winrm", "psexec"],
    ("windows", "linux"): ["ssh"],
    ("linux", "windows"): ["smb", "winrm"],
    ("linux", "linux"): ["ssh"],
}


def _detect_platform(os_str):
    if not os_str:
        return "unknown"
    low = os_str.lower()
    for kw in WINDOWS_KEYWORDS:
        if kw in low:
            return "windows"
    for kw in LINUX_KEYWORDS:
        if kw in low:
            return "linux"
    return "unknown"


def _ip_subnet(ip_str, bits=24):
    try:
        net = ipaddress.ip_network(f"{ip_str}/{bits}", strict=False)
        return str(net.network_address)
    except (ValueError, TypeError):
        return None


def _parse_services(raw):
    if not raw:
        return []
    try:
        data = json.loads(raw)
        if isinstance(data, list):
            return data
    except (json.JSONDecodeError, TypeError):
        pass
    return []


def _is_high_value(username, domain):
    name = (username or "").lower()
    for hv in HIGH_VALUE_USERS:
        if hv in name:
            return True
    dom = (domain or "").lower()
    if "domain admin" in dom or "enterprise admin" in dom:
        return True
    return False


def _open_services(host):
    services = _parse_services(host.get("services") or "")
    return [s for s in services if isinstance(s, dict) and s.get("state") == "open"]


def main():
    data = read_stdin()
    params = data.get("params", {})
    focus_agent_id = params.get("agent_id", "") or data.get("agent_id", "")

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        agents = db.all_agents()
        creds = db.all_credentials()
        hosts = db.all_network_hosts()
        scans = db.all_scan_results()

        if focus_agent_id:
            agents = [a for a in agents if a["id"] == focus_agent_id]

        agent_by_id = {a["id"]: a for a in agents}
        agent_subnets = {}
        for a in agents:
            sub = _ip_subnet(a.get("ip"))
            if sub:
                agent_subnets.setdefault(sub, []).append(a)

        agent_creds_map = {}
        for c in creds:
            aid = c.get("agent_id", "")
            if aid:
                agent_creds_map.setdefault(aid, []).append(c)

        suggestions = []
        method_counts = {"ssh": 0, "wmi": 0, "smb": 0, "winrm": 0, "psexec": 0}

        for agent in agents:
            src_id = agent["id"]
            src_os = _detect_platform(agent.get("os"))
            src_platform = "windows" if src_os == "windows" else "linux"
            src_sub = _ip_subnet(agent.get("ip"))
            src_user = (agent.get("username") or "").lower()
            src_domain = (agent.get("domain") or "").lower()
            src_elevated = agent.get("elevated", False)
            src_integrity = (agent.get("integrity") or "").lower()
            src_is_admin = src_elevated or src_integrity in ("high", "system")
            src_creds = agent_creds_map.get(src_id, [])

            for cred in src_creds:
                cred_user = (cred.get("username") or "").lower()
                cred_domain = (cred.get("domain") or "").lower()
                cred_is_hv = _is_high_value(cred.get("username"), cred.get("domain"))

                for target_agent in agents:
                    if target_agent["id"] == src_id:
                        continue
                    tgt_os = _detect_platform(target_agent.get("os"))
                    tgt_platform = "windows" if tgt_os == "windows" else "linux"
                    tgt_sub = _ip_subnet(target_agent.get("ip"))
                    tgt_user = (target_agent.get("username") or "").lower()
                    tgt_domain = (target_agent.get("domain") or "").lower()

                    if src_sub and tgt_sub and src_sub == tgt_sub:
                        confidence = 0.5
                        if cred_is_hv:
                            confidence = 0.9
                        elif src_user == tgt_user and src_domain == tgt_domain:
                            confidence = 0.8
                        elif src_is_admin:
                            confidence = 0.7

                        method_key = (src_platform, tgt_platform)
                        methods = LATERAL_METHODS.get(method_key, [])
                        if not methods:
                            continue

                        method = methods[0]
                        method_counts[method] = method_counts.get(method, 0) + 1
                        cred_desc = cred.get("username") or "unknown"
                        cred_type = cred.get("type") or "unknown"
                        note = f"Agent {agent.get('hostname') or src_id[:8]} → {target_agent.get('hostname') or target_agent['id'][:8]} via {method.upper()} using {cred_desc} ({cred_type})"
                        if cred_is_hv:
                            note += " [HIGH VALUE]"
                        if src_is_admin:
                            note += " [ELEVATED]"

                        suggestions.append({
                            "source_agent": src_id,
                            "target_agent": target_agent["id"],
                            "target": target_agent.get("ip") or target_agent.get("hostname") or "",
                            "method": method,
                            "credential_id": cred.get("id"),
                            "credential_used": cred_desc,
                            "confidence": round(confidence, 2),
                            "note": note,
                        })

            for target_agent in agents:
                if target_agent["id"] == src_id:
                    continue
                tgt_os = _detect_platform(target_agent.get("os"))
                tgt_platform = "windows" if tgt_os == "windows" else "linux"
                tgt_sub = _ip_subnet(target_agent.get("ip"))

                if src_sub and tgt_sub and src_sub != tgt_sub:
                    continue
                if src_sub and tgt_sub and src_sub == tgt_sub:
                    continue

                method_key = (src_platform, tgt_platform)
                methods = LATERAL_METHODS.get(method_key, [])
                if not methods or not src_creds:
                    continue

                for cred in src_creds:
                    method = methods[0]
                    method_counts[method] = method_counts.get(method, 0) + 1
                    cred_desc = cred.get("username") or "unknown"
                    note = f"Agent {agent.get('hostname') or src_id[:8]} → {target_agent.get('hostname') or target_agent['id'][:8]} via {method.upper()} using {cred_desc} (cross-subnet)"
                    suggestions.append({
                        "source_agent": src_id,
                        "target_agent": target_agent["id"],
                        "target": target_agent.get("ip") or target_agent.get("hostname") or "",
                        "method": method,
                        "credential_id": cred.get("id"),
                        "credential_used": cred_desc,
                        "confidence": 0.4,
                        "note": note,
                    })

        for host in hosts:
            host_ip = host.get("ip")
            if not host_ip:
                continue
            host_sub = _ip_subnet(host_ip)
            host_os = _detect_platform(host.get("os"))
            host_platform = "windows" if host_os == "windows" else "linux"
            open_svcs = _open_services(host)
            has_ssh = any(s.get("service") in ("ssh", "openssh") for s in open_svcs)
            has_smb = any(s.get("service") in ("smb", "microsoft-ds", "netbios-ssn") for s in open_svcs)
            has_wmi = any(s.get("port") == 135 for s in open_svcs)

            for agent in agents:
                agent_sub = _ip_subnet(agent.get("ip"))
                if not agent_sub or not host_sub or agent_sub != host_sub:
                    continue

                agent_os = _detect_platform(agent.get("os"))
                agent_platform = "windows" if agent_os == "windows" else "linux"
                agent_creds = agent_creds_map.get(agent["id"], [])

                for cred in agent_creds:
                    cred_user = cred.get("username") or ""
                    cred_type = cred.get("type") or "unknown"
                    method_key = (agent_platform, host_platform)
                    methods = LATERAL_METHODS.get(method_key, [])

                    for method in methods:
                        usable = False
                        if method == "ssh" and has_ssh:
                            usable = True
                        elif method == "smb" and has_smb:
                            usable = True
                        elif method == "wmi" and has_wmi and agent_platform == "windows" and host_platform == "windows":
                            usable = True
                        elif method in ("winrm", "psexec") and agent_platform == "windows" and host_platform == "windows":
                            usable = True

                        if not usable:
                            continue

                        confidence = 0.6
                        if _is_high_value(cred.get("username"), cred.get("domain")):
                            confidence = 0.85

                        method_counts[method] = method_counts.get(method, 0) + 1
                        note = f"Agent {agent.get('hostname') or agent['id'][:8]} → Host {host.get('hostname') or host_ip} via {method.upper()} using {cred_user} ({cred_type})"
                        suggestions.append({
                            "source_agent": agent["id"],
                            "target_host_id": host["id"],
                            "target": host_ip,
                            "method": method,
                            "credential_id": cred.get("id"),
                            "credential_used": cred_user,
                            "confidence": round(confidence, 2),
                            "note": note,
                        })

        suggestions.sort(key=lambda s: -s["confidence"])

        agents_by_subnet = {}
        for sub, ags in agent_subnets.items():
            agents_by_subnet[sub] = [a["id"] for a in ags]

        credential_overlap = {}
        for aid, clist in agent_creds_map.items():
            for c in clist:
                key = (c.get("username") or "").lower()
                if key:
                    credential_overlap.setdefault(key, set()).add(aid)
        credential_overlap = {k: list(v) for k, v in credential_overlap.items() if len(v) > 1}

        privileged_agents = [
            a["id"] for a in agents
            if a.get("elevated") or (a.get("integrity") or "").lower() in ("high", "system")
        ]

        high_confidence = sum(1 for s in suggestions if s["confidence"] >= 0.7)

        summary = {
            "total_agents": len(agents),
            "total_suggestions": len(suggestions),
            "high_confidence": high_confidence,
            "methods": {k: v for k, v in method_counts.items() if v > 0},
        }

        output_lines = [
            f"Lateral movement analysis for {len(agents)} agent(s)",
            f"Total suggestions: {len(suggestions)} ({high_confidence} high confidence)",
            f"Privileged agents: {len(privileged_agents)}",
        ]
        for s in suggestions[:10]:
            conf_label = "HIGH" if s["confidence"] >= 0.7 else "MED" if s["confidence"] >= 0.5 else "LOW"
            output_lines.append(f"  [{conf_label} {s['confidence']:.0%}] {s['note']}")

        if len(suggestions) > 10:
            output_lines.append(f"  ... and {len(suggestions) - 10} more")

        write_result(
            True,
            output="\n".join(output_lines),
            data={
                "suggestions": suggestions,
                "topology": {
                    "agents_by_subnet": agents_by_subnet,
                    "credential_overlap": credential_overlap,
                    "privileged_agents": privileged_agents,
                },
                "summary": summary,
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
