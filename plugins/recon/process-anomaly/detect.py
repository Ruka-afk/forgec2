#!/usr/bin/env python3
"""Process Anomaly Detection plugin — detects miners, RATs, anti-analysis tools, and suspicious activity."""

import json
import os
import re
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

# ── Pattern definitions ────────────────────────────────────────────────
# Each category maps to a list of (compiled_regex, evidence_label, risk_level).
# risk_level: critical > high > medium > low

MINER_PATTERNS = [
    (re.compile(r"\bxmrig\b", re.I), "xmrig miner", "critical"),
    (re.compile(r"\bminerd\b", re.I), "minerd miner", "critical"),
    (re.compile(r"\bcpuminer\b", re.I), "cpuminer", "critical"),
    (re.compile(r"\bcgminer\b", re.I), "cgminer", "critical"),
    (re.compile(r"\bbfgminer\b", re.I), "bfgminer", "critical"),
    (re.compile(r"\bethminer\b", re.I), "ethminer (Ethereum)", "critical"),
    (re.compile(r"\bphoenixminer\b", re.I), "PhoenixMiner", "critical"),
    (re.compile(r"\bt-rex\b", re.I), "T-Rex miner", "critical"),
    (re.compile(r"\blolminer\b", re.I), "lolMiner", "critical"),
    (re.compile(r"\bnbminer\b", re.I), "NBMiner", "critical"),
    (re.compile(r"\bsrbminer\b", re.I), "SRBMiner", "critical"),
    (re.compile(r"\bkthreadd\b", re.I), "kthreadd (possible disguise)", "high"),
    (re.compile(r"\bstratum\+tcp://", re.I), "stratum mining pool connection", "critical"),
    (re.compile(r"\bhashrate\b", re.I), "hashrate indicator", "high"),
    (re.compile(r"\bcrypto.*pool\b|\bmining.*pool\b", re.I), "mining pool reference", "high"),
]

RAT_PATTERNS = [
    (re.compile(r"\bmetasploit\b", re.I), "Metasploit framework", "critical"),
    (re.compile(r"\bmeterpreter\b", re.I), "Meterpreter payload", "critical"),
    (re.compile(r"\bcobalt[\s_-]?strike\b", re.I), "Cobalt Strike", "critical"),
    (re.compile(r"\bcovenant\b", re.I), "Covenant C2", "critical"),
    (re.compile(r"\bemp(?:ire)?\b", re.I), "Empire C2", "critical"),
    (re.compile(r"\bsliver\b", re.I), "Sliver C2", "critical"),
    (re.compile(r"\bhavoc\b", re.I), "Havoc C2", "critical"),
    (re.compile(r"\bbrute[\s_-]?ratel\b", re.I), "Brute Ratel C4", "critical"),
    (re.compile(r"\bquasar[\s_-]?rat\b", re.I), "Quasar RAT", "critical"),
    (re.compile(r"\basyncrat\b", re.I), "AsyncRAT", "critical"),
    (re.compile(r"\bnjrat\b", re.I), "njRAT", "critical"),
    (re.compile(r"\bgh0st\b", re.I), "Gh0st RAT", "critical"),
    (re.compile(r"\bbeacon(?:\.exe)?\b", re.I), "Cobalt Strike Beacon", "critical"),
    (re.compile(r"\brunpe\b", re.I), "process hollowing indicator", "high"),
    (re.compile(r"\binject(?:ion|or|process)\b", re.I), "process injection indicator", "high"),
]

ANTI_ANALYSIS_PATTERNS = [
    (re.compile(r"\bwireshark\b", re.I), "Wireshark", "high"),
    (re.compile(r"\bprocmon(?:64)?\b", re.I), "Process Monitor", "medium"),
    (re.compile(r"\bprocesshacker\b", re.I), "Process Hacker", "medium"),
    (re.compile(r"\bx64dbg\b", re.I), "x64dbg debugger", "medium"),
    (re.compile(r"\bollydbg\b", re.I), "OllyDbg debugger", "medium"),
    (re.compile(r"\bida(?:q64|q|apro)?\b", re.I), "IDA Pro disassembler", "medium"),
    (re.compile(r"\bimmunity\b", re.I), "Immunity Debugger", "medium"),
    (re.compile(r"\bfiddler\b", re.I), "Fiddler proxy", "high"),
    (re.compile(r"\bcharles\b", re.I), "Charles Proxy", "high"),
    (re.compile(r"\btcpdump\b", re.I), "tcpdump sniffer", "high"),
    (re.compile(r"\bvmtoolsd\b", re.I), "VMware Tools (VM detected)", "low"),
    (re.compile(r"\bvboxservice\b", re.I), "VirtualBox Guest Additions (VM detected)", "low"),
    (re.compile(r"\bvmwaretray\b", re.I), "VMware tray icon", "low"),
    (re.compile(r"\bsandboxie\b|\bsbie(?:cmd|svc)?\b", re.I), "Sandboxie (sandbox detected)", "low"),
    (re.compile(r"\bwireshark.*capture|dumpcap\b", re.I), "packet capture tool", "high"),
    (re.compile(r"\bregshot\b", re.I), "Regshot (registry snapshot tool)", "medium"),
    (re.compile(r"\bapimonitor\b", re.I), "API Monitor", "medium"),
    (re.compile(r"\bdnspy\b", re.I), "dnSpy (.NET debugger)", "medium"),
]

KEYLOGGER_PATTERNS = [
    (re.compile(r"\bkeylogger\b", re.I), "keylogger", "critical"),
    (re.compile(r"\bGetAsyncKeyState\b", re.I), "GetAsyncKeyState hook", "high"),
    (re.compile(r"\bSetWindowsHookEx\b", re.I), "SetWindowsHookEx (keyboard hook)", "high"),
    (re.compile(r"\bSetCapture\b.*keyboard|\bkeyboard.*hook\b", re.I), "keyboard capture hook", "high"),
    (re.compile(r"\bLogKeys\b|\bkeylog\b", re.I), "keylogging tool", "critical"),
]

SUSPICIOUS_PATTERNS = [
    (re.compile(r"powershell.*-[Ee]nc(?:odedCommand)?\s+[A-Za-z0-9+/=]{20,}", re.I), "encoded PowerShell command", "high"),
    (re.compile(r"Invoke-WebRequest.*\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b", re.I), "Invoke-WebRequest to IP address", "high"),
    (re.compile(r"Invoke-WebRequest.*https?://[^\s]+", re.I), "Invoke-WebRequest download", "medium"),
    (re.compile(r"\bcertutil\b.*-[Dd]ecode|\bcertutil\b.*-urlcache", re.I), "certutil decode/cache", "high"),
    (re.compile(r"\bbitsadmin\b.*-[Tt]ransfer|\bbitsadmin\b.*/[Dd]ownload", re.I), "bitsadmin file transfer", "high"),
    (re.compile(r"curl\s+.*-[oO]\s+\S+\s+https?://", re.I), "curl download to file", "medium"),
    (re.compile(r"wget\s+.*https?://", re.I), "wget download", "medium"),
    (re.compile(r"\bmshta\b.*vbscript:", re.I), "mshta VBScript execution", "critical"),
    (re.compile(r"\bwscript\b.*\.vbs|\bcscript\b.*\.vbs", re.I), "VBScript execution", "high"),
    (re.compile(r"\brundll32\b.*javascript:", re.I), "rundll32 JavaScript execution", "critical"),
    (re.compile(r"\bregsvr32\b.*\/[Ss]\s+.*scrobj", re.I), "regsvr32 scriptlet execution", "critical"),
]

CATEGORY_MAP = {
    "miner": MINER_PATTERNS,
    "rat": RAT_PATTERNS,
    "anti_analysis": ANTI_ANALYSIS_PATTERNS,
    "keylogger": KEYLOGGER_PATTERNS,
    "suspicious": SUSPICIOUS_PATTERNS,
}

# Risk weights for summary criticality calculation
RISK_WEIGHT = {"critical": 4, "high": 3, "medium": 2, "low": 1}

# Task types and command patterns that indicate process listings
PROCESS_LIST_TASK_TYPES = {"ps", "shell"}
PROCESS_LIST_CMD_RE = re.compile(
    r"(tasklist|taskim|ps\s+(?:aux|ef|-[aefuwx]*)|top\s+-[bn]|\bGet-Process\b|"
    r"Get-WmiObject\s+Win32_Process|Select-Object\s+Name,\s*Id)",
    re.I,
)


def _extract_process_text(task):
    """Return the combined text of task command + result for scanning."""
    parts = []
    if task.get("command"):
        parts.append(task["command"])
    if task.get("result"):
        parts.append(task["result"])
    return "\n".join(parts)


def _try_parse_process_name(line):
    """Best-effort extraction of a process name from a process-list line."""
    # PowerShell Get-Process: line like "Idle    0   16   ..."
    # tasklist: line like "System Idle Process    0   Console    1    16 K"
    # ps aux: line like "root      1  0.0  0.0 ... /sbin/init"
    stripped = line.strip()
    if not stripped:
        return None, None

    # ps aux format — process name is last column (after last space-segment with /)
    parts = re.split(r"\s+", stripped)
    if len(parts) >= 11:
        # ps aux: COMMAND is field 11+
        cmd = " ".join(parts[10:])
        name = os.path.basename(cmd.split()[0]) if cmd else None
        pid = parts[1]
        return name, pid

    # tasklist CSV or whitespace
    # Try to grab first token as name and a numeric token as PID
    name = parts[0] if parts else None
    pid = None
    for p in parts[1:]:
        if p.isdigit():
            pid = p
            break
    return name, pid


def scan_text_for_anomalies(text):
    """Scan raw text against all anomaly categories; return list of matches."""
    anomalies = []
    for category, patterns in CATEGORY_MAP.items():
        for pattern, evidence_label, risk_level in patterns:
            for m in pattern.finditer(text):
                # Try to grab surrounding context (process name from same line)
                start = text.rfind("\n", 0, m.start()) + 1
                end = text.find("\n", m.end())
                if end == -1:
                    end = len(text)
                line = text[start:end]
                proc_name, pid = _try_parse_process_name(line)
                anomalies.append({
                    "process_name": proc_name or m.group(0),
                    "pid": pid or "",
                    "category": category,
                    "risk_level": risk_level,
                    "evidence": evidence_label,
                    "matched_text": line.strip()[:200],
                })
    return anomalies


def analyze_agent(agent, tasks):
    """Analyze a single agent's tasks for anomalous processes."""
    anomalies = []
    for task in tasks:
        if task.get("status") != "completed":
            continue
        # Only scan tasks that look like they contain process listings
        cmd = task.get("command", "") or ""
        task_type = task.get("type", "")
        is_ps_task = task_type in PROCESS_LIST_TASK_TYPES
        is_ps_cmd = bool(PROCESS_LIST_CMD_RE.search(cmd))

        if not is_ps_task and not is_ps_cmd:
            # Also scan shell tasks whose result mentions process-like output
            result = task.get("result", "") or ""
            if not re.search(r"(PID|ProcessName|Image Name|ppid|COMMAND)", result, re.I):
                continue

        text = _extract_process_text(task)
        if not text:
            continue

        found = scan_text_for_anomalies(text)
        for a in found:
            a["task_id"] = task.get("id", "")
            a["task_created_at"] = task.get("created_at", "")
        anomalies.extend(found)

    # Deduplicate by (process_name, pid, category)
    seen = set()
    unique = []
    for a in anomalies:
        key = (a["process_name"], a["pid"], a["category"])
        if key not in seen:
            seen.add(key)
            unique.append(a)
    return unique


def main():
    data = read_stdin()
    params = data.get("params", {})
    agent_id = params.get("agent_id", "")

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        agents = db.all_agents()
        all_tasks = db.all_tasks()

        if agent_id:
            agent = db.agent_by_id(agent_id)
            if not agent:
                write_result(False, error=f"Agent {agent_id} not found")
                return
            agents = [agent]

        # Group tasks by agent
        tasks_by_agent = {}
        for task in all_tasks:
            aid = task.get("agent_id", "")
            tasks_by_agent.setdefault(aid, []).append(task)

        all_anomalies = []
        by_category = {"miner": 0, "rat": 0, "anti_analysis": 0, "keylogger": 0, "suspicious": 0}
        critical_count = 0
        agents_with_anomalies = 0
        agent_results = []

        for agent in agents:
            aid = agent.get("id", "")
            agent_tasks = tasks_by_agent.get(aid, [])
            anomalies = analyze_agent(agent, agent_tasks)

            if anomalies:
                agents_with_anomalies += 1

            for a in anomalies:
                cat = a["category"]
                if cat in by_category:
                    by_category[cat] += 1
                if a["risk_level"] == "critical":
                    critical_count += 1

            all_anomalies.extend(anomalies)

            agent_results.append({
                "agent_id": aid,
                "hostname": agent.get("hostname", ""),
                "ip": agent.get("ip", ""),
                "status": agent.get("status", ""),
                "anomaly_count": len(anomalies),
                "anomalies": anomalies,
            })

        total_agents = len(agents)
        total_anomalies = len(all_anomalies)

        output = (
            f"Analyzed {total_agents} agents | "
            f"{total_anomalies} anomalies | "
            f"{critical_count} critical"
        )

        write_result(
            True,
            output=output,
            data={
                "total_agents": total_agents,
                "anomalies_found": total_anomalies,
                "by_category": by_category,
                "critical_count": critical_count,
                "agents_with_anomalies": agents_with_anomalies,
                "agents": agent_results,
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
