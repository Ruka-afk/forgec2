#!/usr/bin/env python3
"""Log Analyzer plugin — parses Windows Event Logs and Linux syslog from task results for security events."""

import json
import os
import re
import sys
from collections import defaultdict

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

WINDOWS_SECURITY_IDS = {
    "4624": {"type": "logon", "risk": "info", "detail": "Logon success"},
    "4625": {"type": "failed_logon", "risk": "high", "detail": "Failed logon attempt"},
    "4648": {"type": "explicit_creds", "risk": "high", "detail": "Explicit credential logon (runas/PsExec)"},
    "4672": {"type": "special_privileges", "risk": "medium", "detail": "Special privileges assigned to new logon"},
    "4688": {"type": "process_create", "risk": "info", "detail": "New process created"},
    "4720": {"type": "user_created", "risk": "high", "detail": "User account created"},
    "4726": {"type": "user_deleted", "risk": "high", "detail": "User account deleted"},
    "4732": {"type": "group_member_added", "risk": "high", "detail": "Member added to local group"},
    "1102": {"type": "audit_log_cleared", "risk": "critical", "detail": "Audit log cleared"},
}

LOGON_TYPE_NAMES = {
    "2": "Interactive",
    "3": "Network",
    "4": "Batch",
    "5": "Service",
    "7": "Unlock",
    "8": "NetworkCleartext",
    "9": "NewCredentials",
    "10": "RemoteInteractive",
    "11": "CachedInteractive",
}

SYSMON_IDS = {
    "1": "ProcessCreate",
    "3": "NetworkConnect",
    "7": "ImageLoaded",
    "8": "CreateRemoteThread",
    "10": "ProcessAccess",
    "11": "FileCreate",
    "13": "RegistryValueSet",
    "15": "FileCreateStreamHash",
    "22": "DnsQuery",
    "25": "ProcessTampering",
}

LINUX_PATTERNS = [
    {"regex": r"(?:su|sudo)\s*:\s*(?:FAILED|authentication failure).*for\s+(\S+)", "type": "su_failed", "risk": "high", "detail": "su/sudo authentication failure"},
    {"regex": r"sudo:\s+(\S+)\s*:\s+command not allowed", "type": "sudo_denied", "risk": "medium", "detail": "sudo command not allowed"},
    {"regex": r"sshd\[(\d+)\]:\s+Failed password for (?:invalid user )?(\S+) from (\S+)", "type": "sshd_failed", "risk": "high", "detail": "SSH failed password"},
    {"regex": r"sshd\[(\d+)\]:\s+Accepted (?:password|publickey) for (\S+) from (\S+)", "type": "sshd_accepted", "risk": "info", "detail": "SSH login accepted"},
    {"regex": r"iptables.*-(?:A|D|I)\s+(\S+)", "type": "iptables_change", "risk": "medium", "detail": "iptables rule change"},
    {"regex": r"crontab\[(\d+)\]:\s+\((\S+)\)\s+LIST\s*\((\S+)\)", "type": "crontab_mod", "risk": "medium", "detail": "crontab modification"},
    {"regex": r"kernel:\s+\[.*\]\s+IN=.*OUT=.*SRC=.*DST=.*PROTO=.*DPT=(\d+)", "type": "firewall_drop", "risk": "low", "detail": "Firewall packet drop"},
    {"regex": r"pam_unix\(sshd:auth\):\s+authentication failure.*rhost=(\S+)\s+user=(\S+)", "type": "pam_auth_fail", "risk": "high", "detail": "PAM authentication failure"},
]

EVENT_ID_RE = re.compile(r"(?:Event\s+)?(?:ID|Id|id)[:\s=]+(\d+)", re.IGNORECASE)
WEVTUTIL_LINE_RE = re.compile(
    r"^(Event\[)?(\d+)?\]?|"
    r"Provider\s*\[\s*Name\s*=\s*([^\]]+)\]|"
    r"EventRecordID\s*=\s*(\d+)|"
    r"TimeCreated\s+SystemTime\s*=\s*['\"]?([^'\"}]+)",
    re.IGNORECASE,
)
XML_EVENT_RE = re.compile(r"<Event[^>]*>(.*?)</Event>", re.DOTALL)
XML_FIELD_RE = re.compile(r"<(\w+)\s+[^>]*>([^<]*)</\1>")
POWERSHELL_WIN_EVENT_RE = re.compile(
    r"(?:ProviderName|TimeCreated|Id|LevelDisplayName|Message)\s*:\s*(.+)",
    re.IGNORECASE,
)


def parse_wevtutil_output(text):
    """Parse wevtutil qe XML output or text-formatted output."""
    events = []

    for match in XML_EVENT_RE.finditer(text):
        xml_body = match.group(1)
        fields = {}
        for fm in XML_FIELD_RE.finditer(xml_body):
            fields[fm.group(1)] = fm.group(2).strip()
        if "EventID" in fields:
            events.append({
                "id": fields.get("EventID", ""),
                "time": fields.get("TimeCreated", fields.get("SystemTime", "")),
                "provider": fields.get("Provider", fields.get("ProviderName", "")),
                "level": fields.get("Level", ""),
                "computer": fields.get("Computer", ""),
                "message": fields.get("Message", fields.get("Data", "")),
                "raw": xml_body[:500],
            })

    if not events:
        current = {}
        for line in text.splitlines():
            line = line.strip()
            if not line:
                if current.get("id"):
                    events.append(current)
                current = {}
                continue
            m = re.match(r"^EventID\s*[:=]\s*(\d+)", line, re.IGNORECASE)
            if m:
                current["id"] = m.group(1)
                continue
            m = re.match(r"^TimeCreated\s*[:=]\s*(.+)", line, re.IGNORECASE)
            if m:
                current["time"] = m.group(1).strip().strip("\"'")
                continue
            m = re.match(r"^Provider\s*[:=]\s*(.+)", line, re.IGNORECASE)
            if m:
                current["provider"] = m.group(1).strip()
                continue
            m = re.match(r"^Message\s*[:=]\s*(.+)", line, re.IGNORECASE)
            if m:
                current["message"] = m.group(1).strip()
                continue
            if not current.get("id"):
                eid = EVENT_ID_RE.search(line)
                if eid:
                    current["id"] = eid.group(1)
        if current.get("id"):
            events.append(current)

    return events


def parse_powershell_winevent(text):
    """Parse Get-WinEvent formatted output (Format-List style)."""
    events = []
    blocks = re.split(r"\n\s*---\s*\n|\n\s*\n(?=ProviderName|TimeCreated)", text)

    for block in blocks:
        if not block.strip():
            continue
        fields = {}
        for m in POWERSHELL_WIN_EVENT_RE.finditer(block):
            fields[m.group(0).split(":")[0].strip()] = m.group(1).strip()
        if not fields:
            id_m = EVENT_ID_RE.search(block)
            if id_m:
                fields["Id"] = id_m.group(1)

        if "Id" in fields or "EventID" in fields:
            events.append({
                "id": fields.get("Id", fields.get("EventID", "")),
                "time": fields.get("TimeCreated", ""),
                "provider": fields.get("ProviderName", ""),
                "level": fields.get("LevelDisplayName", ""),
                "message": fields.get("Message", ""),
                "raw": block[:500],
            })

    if not events:
        for m in re.finditer(r"EventRecordID\s*:\s*(\d+).*?Id\s*:\s*(\d+)", text, re.DOTALL):
            events.append({
                "id": m.group(2),
                "time": "",
                "provider": "",
                "level": "",
                "message": "",
                "raw": m.group(0)[:300],
            })

    return events


def parse_linux_logs(text):
    """Parse Linux syslog / auth.log / secure entries."""
    events = []
    for line in text.splitlines():
        for pat_def in LINUX_PATTERNS:
            m = re.search(pat_def["regex"], line)
            if m:
                events.append({
                    "id": pat_def["type"],
                    "time": line[:16] if len(line) > 16 else line,
                    "type": pat_def["type"],
                    "risk": pat_def["risk"],
                    "detail": pat_def["detail"],
                    "matches": list(m.groups()),
                    "raw": line[:300],
                })
                break
    return events


def analyze_windows_event(raw_event):
    """Classify a single Windows event and extract security-relevant info."""
    eid = str(raw_event.get("id", "")).strip()
    info = WINDOWS_SECURITY_IDS.get(eid)
    if not info:
        return None

    message = raw_event.get("message", "")
    risk = info["risk"]
    detail = info["detail"]

    if eid == "4624":
        logon_type_match = re.search(r"Logon Type\s*:\s*(\d+)", message)
        logon_type = logon_type_match.group(1) if logon_type_match else ""
        type_name = LOGON_TYPE_NAMES.get(logon_type, f"Type {logon_type}")
        src_ip_match = re.search(r"Source Network Address\s*:\s*(\S+)", message)
        src_ip = src_ip_match.group(1) if src_ip_match else ""
        target_user_match = re.search(r"TargetUserName\s*:\s*(\S+)", message)
        target_user = target_user_match.group(1) if target_user_match else ""

        if logon_type == "10":
            risk = "high"
            detail = f"RDP logon ({type_name}) — lateral movement indicator"
        elif logon_type == "3":
            risk = "medium"
            detail = f"Network logon ({type_name}) — possible lateral movement"
        elif logon_type == "9":
            risk = "medium"
            detail = f"NewCredentials ({type_name}) — runas / net use possible"
        else:
            detail = f"Logon ({type_name})"

        return {
            "id": eid,
            "time": raw_event.get("time", ""),
            "agent": raw_event.get("computer", ""),
            "type": info["type"],
            "detail": f"{detail} | user={target_user} src={src_ip}",
            "risk": risk,
            "logon_type": logon_type,
        }

    if eid == "4625":
        src_ip_match = re.search(r"Source Network Address\s*:\s*(\S+)", message)
        src_ip = src_ip_match.group(1) if src_ip_match else ""
        target_user_match = re.search(r"TargetUserName\s*:\s*(\S+)", message)
        target_user = target_user_match.group(1) if target_user_match else ""
        return {
            "id": eid,
            "time": raw_event.get("time", ""),
            "agent": raw_event.get("computer", ""),
            "type": info["type"],
            "detail": f"Failed logon | user={target_user} src={src_ip}",
            "risk": risk,
        }

    if eid == "4688":
        proc_match = re.search(r"New Process Name\s*:\s*(\S+)", message)
        proc_name = proc_match.group(1) if proc_match else ""
        cmd_match = re.search(r"Process Command Line\s*:\s*(.+)", message)
        cmd_line = cmd_match.group(1).strip() if cmd_match else ""
        if cmd_line:
            risk = "medium"
            detail = f"Process with command line | {proc_name}"
        return {
            "id": eid,
            "time": raw_event.get("time", ""),
            "agent": raw_event.get("computer", ""),
            "type": info["type"],
            "detail": f"{detail} | cmd={cmd_line[:120]}",
            "risk": risk,
        }

    if eid in ("4720", "4726"):
        target_match = re.search(r"TargetUserName\s*:\s*(\S+)", message)
        target = target_match.group(1) if target_match else ""
        return {
            "id": eid,
            "time": raw_event.get("time", ""),
            "agent": raw_event.get("computer", ""),
            "type": info["type"],
            "detail": f"{info['detail']} | user={target}",
            "risk": risk,
        }

    if eid == "4732":
        member_match = re.search(r"MemberName\s*:\s*(\S+)", message)
        member = member_match.group(1) if member_match else ""
        group_match = re.search(r"TargetUserName\s*:\s*(\S+)", message)
        group = group_match.group(1) if group_match else ""
        return {
            "id": eid,
            "time": raw_event.get("time", ""),
            "agent": raw_event.get("computer", ""),
            "type": info["type"],
            "detail": f"{info['detail']} | member={member} group={group}",
            "risk": risk,
        }

    if eid == "1102":
        return {
            "id": eid,
            "time": raw_event.get("time", ""),
            "agent": raw_event.get("computer", ""),
            "type": info["type"],
            "detail": "ANTI-FORENSICS: Audit log cleared",
            "risk": "critical",
        }

    return {
        "id": eid,
        "time": raw_event.get("time", ""),
        "agent": raw_event.get("computer", ""),
        "type": info["type"],
        "detail": info["detail"],
        "risk": risk,
    }


def extract_sysmon_event(raw_event):
    """Extract Sysmon events (IDs 1,3,7,8,10,11,13,15,22,25)."""
    eid = str(raw_event.get("id", "")).strip()
    if eid not in SYSMON_IDS:
        return None
    message = raw_event.get("message", "")
    return {
        "id": eid,
        "time": raw_event.get("time", ""),
        "agent": raw_event.get("computer", ""),
        "type": f"sysmon_{SYSMON_IDS[eid]}",
        "detail": f"Sysmon {SYSMON_IDS[eid]}: {message[:150]}",
        "risk": "info",
    }


def detect_brute_force(events):
    """Detect brute force from repeated 4625 failures from same source."""
    failed_by_src = defaultdict(int)
    for e in events:
        if e.get("id") == "4625" or e.get("type") == "sshd_failed":
            src = ""
            if "src=" in e.get("detail", ""):
                m = re.search(r"src=(\S+)", e["detail"])
                if m:
                    src = m.group(1)
            elif e.get("matches") and len(e["matches"]) >= 3:
                src = e["matches"][2]
            if src:
                failed_by_src[src] += 1
    return {src: cnt for src, cnt in failed_by_src.items() if cnt >= 5}


def detect_lateral_movement(events):
    """Detect lateral movement indicators."""
    indicators = []
    for e in events:
        risk = e.get("risk", "")
        etype = e.get("type", "")
        if etype in ("explicit_creds",) or "RDP" in e.get("detail", "") or "lateral" in e.get("detail", "").lower():
            indicators.append(e)
        if etype == "logon" and e.get("logon_type") in ("3", "9", "10"):
            indicators.append(e)
        if etype in ("sshd_accepted",):
            indicators.append(e)
    return indicators


def main():
    data = read_stdin()
    agent_id = data.get("agent_id") or data.get("params", {}).get("agent_id", "")
    max_events = data.get("params", {}).get("max_events", 500)
    if isinstance(max_events, str):
        try:
            max_events = int(max_events)
        except ValueError:
            max_events = 500

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        tasks = db.tasks_for_agent(agent_id) if agent_id else db.all_tasks()
        log_tasks = [
            t for t in tasks
            if t.get("status") == "completed"
            and any(
                kw in (t.get("command", "") or "").lower()
                for kw in (
                    "wevtutil", "get-winevent", "winevent", "eventlog",
                    "event log", "cat /var/log", "journalctl", "auth.log",
                    "syslog", "/var/log/secure", "dmesg",
                )
            )
        ]

        all_events = []
        raw_logs_parsed = 0
        log_sources = set()

        for task in log_tasks[:30]:
            result_text = task.get("result", "")
            if not result_text:
                continue
            raw_logs_parsed += 1
            command = (task.get("command", "") or "").lower()
            agent = task.get("agent_id", "")

            if "wevtutil" in command:
                log_sources.add("wevtutil")
                raw_events = parse_wevtutil_output(result_text)
                for re_ in raw_events:
                    re_["computer"] = agent
                    classified = analyze_windows_event(re_)
                    if classified:
                        all_events.append(classified)
                    else:
                        sysmon = extract_sysmon_event(re_)
                        if sysmon:
                            sysmon["agent"] = agent
                            all_events.append(sysmon)

            elif "get-winevent" in command or "winevent" in command:
                log_sources.add("powershell_winevent")
                raw_events = parse_powershell_winevent(result_text)
                for re_ in raw_events:
                    re_["computer"] = agent
                    classified = analyze_windows_event(re_)
                    if classified:
                        all_events.append(classified)
                    else:
                        sysmon = extract_sysmon_event(re_)
                        if sysmon:
                            sysmon["agent"] = agent
                            all_events.append(sysmon)

            elif any(kw in command for kw in ("cat /var/log", "journalctl", "syslog", "auth.log", "/var/log/secure", "dmesg")):
                log_sources.add("linux_syslog")
                linux_events = parse_linux_logs(result_text)
                for le in linux_events:
                    le["agent"] = agent
                all_events.extend(linux_events)

            elif "event" in command:
                log_sources.add("wevtutil")
                raw_events = parse_wevtutil_output(result_text)
                for re_ in raw_events:
                    re_["computer"] = agent
                    classified = analyze_windows_event(re_)
                    if classified:
                        all_events.append(classified)
                    else:
                        sysmon = extract_sysmon_event(re_)
                        if sysmon:
                            sysmon["agent"] = agent
                            all_events.append(sysmon)

        if len(all_events) > max_events:
            critical = [e for e in all_events if e.get("risk") in ("critical", "high")]
            remaining = [e for e in all_events if e.get("risk") not in ("critical", "high")]
            all_events = critical + remaining[: max_events - len(critical)]

        by_type = defaultdict(int)
        for e in all_events:
            by_type[e.get("type", "unknown")] += 1

        anti_forensics = any(e.get("type") == "audit_log_cleared" or e.get("risk") == "critical" for e in all_events)
        brute_force = detect_brute_force(all_events)
        brute_force_detected = len(brute_force) > 0
        lateral = detect_lateral_movement(all_events)
        lateral_movement = len(lateral) > 0

        critical_count = sum(1 for e in all_events if e.get("risk") in ("critical", "high"))

        summary = {
            "total": len(all_events),
            "by_type": dict(by_type),
            "anti_forensics": anti_forensics,
            "brute_force_detected": brute_force_detected,
            "brute_force_sources": brute_force,
            "lateral_movement": lateral_movement,
            "lateral_movement_count": len(lateral),
            "log_sources": sorted(log_sources),
            "logs_scanned": raw_logs_parsed,
        }

        output = (
            f"Analyzed {raw_logs_parsed} logs | "
            f"{len(all_events)} security events | "
            f"{critical_count} critical indicators"
        )
        if anti_forensics:
            output += " | ANTI-FORENSICS DETECTED"
        if brute_force_detected:
            output += f" | Brute force from {len(brute_force)} sources"
        if lateral_movement:
            output += f" | Lateral movement ({len(lateral)} indicators)"

        write_result(True, output=output, data={"events": all_events, "summary": summary})
    finally:
        db.close()


if __name__ == "__main__":
    main()
