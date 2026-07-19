#!/usr/bin/env python3
"""Memory Indicators plugin — detects credential dumping tools, code injection, and suspicious DLL loads."""

import re
import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

# ── Indicator definitions ──────────────────────────────────────────────

CRITICAL_INDICATORS = [
    (re.compile(r"\bOpenProcess\b.*(?:lsass|PID\s*\d+)", re.I), "lsass_handle", "CRITICAL",
     "Process opened handle to lsass.exe (credential dumping)"),
    (re.compile(r"\bProcess.*lsass.*(?:Open|Duplicate|Query)", re.I), "lsass_handle", "CRITICAL",
     "lsass.exe handle operation detected"),
    (re.compile(r"\bmimikatz(?:64)?\.exe\b", re.I), "mimikatz", "CRITICAL",
     "Mimikatz executable detected"),
    (re.compile(r"\bmimilib\.dll\b", re.I), "mimikatz", "CRITICAL",
     "Mimikatz library (mimilib.dll) detected"),
    (re.compile(r"\bsekurlsa::\b", re.I), "mimikatz", "CRITICAL",
     "Mimikatz sekurlsa module command detected"),
    (re.compile(r"\bcomsvcs\.dll\b.*(?:MiniDump|Load)", re.I), "comsvcs_minidump", "CRITICAL",
     "comsvcs.dll MiniDump abuse (LSASS dump via COM+ service)"),
    (re.compile(r"\bcomsvcs\.dll\b", re.I), "comsvcs_minidump", "MEDIUM",
     "comsvcs.dll loaded (potential MiniDump abuse)"),
    (re.compile(r"\brundll32\.exe\b.*comsvcs", re.I), "comsvcs_minidump", "CRITICAL",
     "rundll32 loading comsvcs.dll (classic LSASS dump technique)"),
]

HIGH_INDICATORS = [
    (re.compile(r"\bprocdump(?:64)?\.exe\b.*(?:lsass|-[Mm])", re.I), "procdump_lsass", "HIGH",
     "procdump targeting lsass.exe"),
    (re.compile(r"\bprocdump(?:64)?\.exe\b", re.I), "procdump", "MEDIUM",
     "procdump detected (memory dump utility)"),
    (re.compile(r"\bsqldumper\.exe\b", re.I), "sqldumper", "MEDIUM",
     "sqldumper.exe detected (SQL debug dump tool, abused for credential dumping)"),
    (re.compile(r"\bdump(?:_)?(?:lsass|cred|token)\b", re.I), "credential_dump", "HIGH",
     "Credential dump command detected"),
    (re.compile(r"\bMiniDumpWriteDump\b", re.I), "minidump_api", "HIGH",
     "MiniDumpWriteDump API call (LSASS memory dump)"),
    (re.compile(r"\bWriteProcessMemory\b.*(?:0x10000|0x40000)", re.I), "shellcode_inject", "HIGH",
     "WriteProcessMemory with RWX flags (shellcode injection)"),
    (re.compile(r"\bVirtualAllocEx\b.*(?:MEM_COMMIT|0x1000)\b", re.I), "shellcode_inject", "HIGH",
     "VirtualAllocEx in remote process (code injection)"),
    (re.compile(r"\bNtCreateThreadEx\b", re.I), "thread_injection", "HIGH",
     "NtCreateThreadEx call (remote thread injection)"),
    (re.compile(r"\bCreateRemoteThread\b", re.I), "thread_injection", "HIGH",
     "CreateRemoteThread detected (process injection)"),
    (re.compile(r"\bResumeThread\b", re.I), "thread_injection", "HIGH",
     "ResumeThread on foreign thread (suspended injection)"),
    (re.compile(r"\bReadProcessMemory\b.*lsass", re.I), "lsass_read", "HIGH",
     "ReadProcessMemory on lsass.exe"),
    (re.compile(r"\bZwQueryInformationProcess\b.*(?:ProcessDebugPort|DebugObjectHandle)", re.I),
     "debug_query", "HIGH", "Anti-debug process query detected"),
]

MEDIUM_INDICATORS = [
    (re.compile(r"\bdbghelp\.dll\b", re.I), "dbghelp_load", "MEDIUM",
     "dbghelp.dll loaded (debug/dump library, abnormal outside debuggers)"),
    (re.compile(r"\bdbgcore\.dll\b", re.I), "dbgcore_load", "MEDIUM",
     "dbgcore.dll loaded (crash dump library, credential dumping indicator)"),
    (re.compile(r"\brasdl\.dll\b", re.I), "rasdll_load", "MEDIUM",
     "rasdl.dll loaded (RAS debug library)"),
    (re.compile(r"\bamsi\.dll\b.*(?:Patch|Bypass|Uninitialize)", re.I), "amsi_bypass", "HIGH",
     "AMSI bypass attempt detected"),
    (re.compile(r"\bEtwEventWrite\b.*(?:Patch|Disable|Bypass)", re.I), "etw_bypass", "HIGH",
     "ETW tampering attempt detected"),
    (re.compile(r"\bNtSetInformationThread\b.*(?:ThreadHideFromDebugger)", re.I),
     "anti_debug", "HIGH", "ThreadHideFromDebugger (anti-debug)"),
]

LOW_INDICATORS = [
    (re.compile(r"\b(?:unsigned|no signature|invalid signature)\b.*\.dll", re.I),
     "unsigned_dll", "LOW", "Unsigned DLL detected"),
    (re.compile(r"\bDLL\b.*(?:load|inject|side-load)", re.I), "dll_load", "LOW",
     "Suspicious DLL load/sideload detected"),
    (re.compile(r"\bdbghelp\.dll\b.*(?:svchost|explorer|csrss)", re.I), "dbghelp_abnormal", "MEDIUM",
     "dbghelp.dll in non-debugger system process"),
    (re.compile(r"\bdbgcore\.dll\b.*(?:svchost|explorer|csrss)", re.I), "dbgcore_abnormal", "MEDIUM",
     "dbgcore.dll in non-debugger system process"),
]

# Processes that legitimately use debug DLLs
LEGITIMATE_DEBUG_PROCESSES = {
    "devenv.exe", "vscode.exe", "x64dbg.exe", "x32dbg.exe", "windbg.exe",
    "ida.exe", "ida64.exe", "ollydbg.exe", "procmon.exe", "procmon64.exe",
    "dnspy.exe", "dnSpy.exe", "windbgx.exe", "cdb.exe", "ntsd.exe",
    "msbuild.exe", "mcs.exe", "csc.exe",
}

# Well-known safe system paths
KNOWN_SAFE_PATHS = {
    r"c:\windows\system32", r"c:\windows\syswow64", r"c:\windows\winsxs",
    r"c:\program files", r"c:\program files (x86)",
    r"c:\programdata\microsoft", r"c:\windows\microsoft.net",
}

# Task types and commands that indicate memory/module inspection tasks
MODULE_SCAN_TASK_TYPES = {"shell", "ps"}
MODULE_SCAN_CMD_RE = re.compile(
    r"(tasklist\s*/[mMm]|Get-Process\s+\w+\s+-Module|"
    r"Get-Process.*Select.*Module|"
    r"Get-Module|Get-CimInstance.*Win32_Module|"
    r"enum-process-modules|NtQueryVirtualMemory|"
    r"VirtualQueryEx|EnumerateLoadedModules)",
    re.I,
)


def _extract_task_text(task):
    """Return combined text of task command + result for scanning."""
    parts = []
    if task.get("command"):
        parts.append(task["command"])
    if task.get("result"):
        parts.append(task["result"])
    return "\n".join(parts)


def _classify_risk(severity):
    """Map severity string to risk level."""
    return severity


def scan_text_for_indicators(text):
    """Scan raw text against all indicator categories; return list of matches."""
    indicators = []
    all_patterns = CRITICAL_INDICATORS + HIGH_INDICATORS + MEDIUM_INDICATORS + LOW_INDICATORS

    for pattern, indicator_type, risk, detail in all_patterns:
        for m in pattern.finditer(text):
            start = text.rfind("\n", 0, m.start()) + 1
            end = text.find("\n", m.end())
            if end == -1:
                end = len(text)
            line = text[start:end].strip()

            # Try to extract process name and PID from the line
            proc_name, pid = _parse_process_from_line(line)

            indicators.append({
                "process": proc_name or m.group(0)[:60],
                "pid": pid or "",
                "indicator_type": indicator_type,
                "risk": risk,
                "detail": detail,
                "evidence": line[:200],
            })

    return indicators


def _parse_process_from_line(line):
    """Best-effort extraction of process name and PID from a line of output."""
    parts = line.split()
    if not parts:
        return None, None

    # tasklist /m format: "name.exe    PID  Session  Session#  Mem Usage"
    # PowerShell Get-Process Module: name  PID  Modules
    for i, part in enumerate(parts):
        if part.isdigit() and i > 0:
            name = parts[i - 1]
            if name.lower().endswith((".exe", ".dll")):
                return name, part
            # Try another position back
            if i >= 2:
                name = parts[i - 2]
                if name.lower().endswith((".exe", ".dll")):
                    return name, part
    return None, None


def _is_dll_in_known_path(evidence):
    """Check if DLL evidence references a known safe system path."""
    ev_lower = evidence.lower()
    return any(sp in ev_lower for sp in KNOWN_SAFE_PATHS)


def _is_debugger_process(evidence):
    """Check if evidence references a legitimate debugger/IDE process."""
    ev_lower = evidence.lower()
    return any(dp in ev_lower for dp in LEGITIMATE_DEBUG_PROCESSES)


def analyze_agent(agent, tasks):
    """Analyze a single agent's tasks for memory forensics indicators."""
    indicators = []
    for task in tasks:
        if task.get("status") != "completed":
            continue

        cmd = task.get("command", "") or ""
        task_type = task.get("type", "")
        is_scan_task = task_type in MODULE_SCAN_TASK_TYPES

        if not is_scan_task and not MODULE_SCAN_CMD_RE.search(cmd):
            result = task.get("result", "") or ""
            if not re.search(r"(Module|DLL|Loaded|Inject|Memory|lsass|mimikatz)", result, re.I):
                continue

        text = _extract_task_text(task)
        if not text:
            continue

        found = scan_text_for_indicators(text)

        # Filter false positives for debug DLLs in legitimate debugger processes
        filtered = []
        for ind in found:
            # If it's a debug DLL load and the process is a known debugger, downgrade to LOW
            if ind["indicator_type"] in ("dbghelp_load", "dbgcore_load"):
                if _is_debugger_process(ind["evidence"]):
                    ind["risk"] = "LOW"
                    ind["detail"] += " (legitimate debugger process — likely benign)"
            # If DLL is from a known safe system path and risk is LOW, keep it but note
            if ind["risk"] == "LOW" and _is_dll_in_known_path(ind["evidence"]):
                ind["detail"] += " (loaded from system path)"
            filtered.append(ind)

            ind["task_id"] = task.get("id", "")

        indicators.extend(filtered)

    # Deduplicate by (process, indicator_type)
    seen = set()
    unique = []
    for ind in indicators:
        key = (ind["process"], ind["indicator_type"])
        if key not in seen:
            seen.add(key)
            unique.append(ind)

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

        tasks_by_agent = {}
        for task in all_tasks:
            aid = task.get("agent_id", "")
            tasks_by_agent.setdefault(aid, []).append(task)

        all_indicators = []
        by_risk = {"critical": 0, "high": 0, "medium": 0, "low": 0}
        credential_dump_detected = False
        agent_results = []

        for agent in agents:
            aid = agent.get("id", "")
            agent_tasks = tasks_by_agent.get(aid, [])
            indicators = analyze_agent(agent, agent_tasks)

            for ind in indicators:
                risk_key = ind["risk"].lower()
                if risk_key in by_risk:
                    by_risk[risk_key] += 1
                if ind["risk"] == "CRITICAL" and ind["indicator_type"] in (
                    "lsass_handle", "mimikatz", "comsvcs_minidump",
                ):
                    credential_dump_detected = True

            all_indicators.extend(indicators)

            agent_results.append({
                "id": aid,
                "hostname": agent.get("hostname", ""),
                "ip": agent.get("ip", ""),
                "status": agent.get("status", ""),
                "indicators": indicators,
            })

        total_agents = len(agents)
        total_indicators = len(all_indicators)

        output = (
            f"Scanned {total_agents} agents | "
            f"{total_indicators} indicators | "
            f"{by_risk['critical']} critical | "
            f"{by_risk['high']} high"
        )

        write_result(
            True,
            output=output,
            data={
                "agents": agent_results,
                "summary": {
                    "total_agents": total_agents,
                    "total_indicators": total_indicators,
                    "by_risk": {
                        "critical": by_risk["critical"],
                        "high": by_risk["high"],
                        "medium": by_risk["medium"],
                        "low": by_risk["low"],
                    },
                    "credential_dump_detected": credential_dump_detected,
                },
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
