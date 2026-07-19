#!/usr/bin/env python3
"""Process Analyzer plugin — parses process list task results and detects security tools."""

import json
import os
import re
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

SECURITY_TOOL_KEYWORDS_DEFAULT = (
    "MsSense,SENSE,cylance,cyltray,symcorp,symantec,mcafee,mcshield,"
    "avp,avg,avpui,cb,cbrps,bulwark,deep-security,osquery,sysmon,"
    "winaudit,procmon,procmon64,wireshark,fiddler,processhacker"
)


def parse_ps_output(text: str) -> list:
    """Parse PowerShell Get-Process or tasklist output into structured rows."""
    processes = []
    lines = text.strip().splitlines()
    if len(lines) < 2:
        return processes

    # Try to detect header columns
    header = lines[0]
    # Normalize whitespace-separated or CSV
    if "," in header:
        cols = [c.strip().lower() for c in header.split(",")]
        for line in lines[1:]:
            parts = [p.strip() for p in line.split(",")]
            if len(parts) >= len(cols):
                row = {cols[i]: parts[i] for i in range(len(cols))}
                processes.append(row)
    else:
        # Whitespace-aligned columns
        # Find column positions from header
        col_starts = []
        col_names = []
        for i, ch in enumerate(header):
            if ch != " " and (i == 0 or header[i - 1] == " "):
                col_starts.append(i)
                # Find end of word
                j = i
                while j < len(header) and header[j] != " ":
                    j += 1
                col_names.append(header[i:j].lower())

        for line in lines[1:]:
            if not line.strip():
                continue
            row = {}
            for ci, start in enumerate(col_starts):
                end = col_starts[ci + 1] if ci + 1 < len(col_starts) else len(line)
                val = line[start:end].strip()
                if ci < len(col_names):
                    row[col_names[ci]] = val
            if row:
                processes.append(row)

    return processes


def detect_security_tools(processes: list, keywords: str) -> list:
    """Detect known security/EDR/AV tools in process list."""
    kw_list = [k.strip().lower() for k in keywords.split(",") if k.strip()]
    found = []
    for proc in processes:
        name = ""
        for key in ("name", "processname", "image"):
            if key in proc:
                name = proc[key].lower()
                break
        if not name:
            continue
        for kw in kw_list:
            if kw in name:
                found.append({
                    "name": proc.get("name") or proc.get("processname") or proc.get("image", ""),
                    "pid": proc.get("id") or proc.get("pid", ""),
                    "matched_keyword": kw,
                })
                break
    return found


def main():
    data = read_stdin()
    agent_id = data.get("agent_id", "")
    params = data.get("params", {})
    config = data.get("config", {})
    keywords = config.get("security_tool_keywords", SECURITY_TOOL_KEYWORDS_DEFAULT)

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        # Find recent ps-type task results for this agent
        tasks = db.tasks_for_agent(agent_id) if agent_id else db.all_tasks()
        ps_tasks = [t for t in tasks if t.get("type") in ("ps", "shell") and t.get("status") == "completed"]

        all_processes = []
        security_tools = []
        sources = []

        for task in ps_tasks[:10]:  # Check last 10 completed tasks
            result_text = task.get("result", "")
            if not result_text:
                continue
            sources.append({
                "task_id": task.get("id"),
                "created_at": task.get("created_at"),
                "command": (task.get("command", "") or "")[:80],
            })
            procs = parse_ps_output(result_text)
            all_processes.extend(procs)
            tools = detect_security_tools(procs, keywords)
            security_tools.extend(tools)

        # Deduplicate security tools by name+pid
        seen = set()
        unique_tools = []
        for t in security_tools:
            key = (t["name"], t["pid"])
            if key not in seen:
                seen.add(key)
                unique_tools.append(t)

        # Process name frequency
        name_freq = {}
        for p in all_processes:
            name = ""
            for key in ("name", "processname", "image"):
                if key in p:
                    name = p[key]
                    break
            if name:
                name_freq[name] = name_freq.get(name, 0) + 1

        top_processes = sorted(name_freq.items(), key=lambda x: -x[1])[:20]

        output_lines = [
            f"Analyzed {len(ps_tasks)} task results for agent {agent_id or 'all'}",
            f"Found {len(all_processes)} process entries",
            f"Security tools detected: {len(unique_tools)}",
        ]
        if unique_tools:
            output_lines.append(
                "WARNING: " + ", ".join(t["name"] for t in unique_tools[:5])
            )

        write_result(
            True,
            output="\n".join(output_lines),
            data={
                "agent_id": agent_id,
                "total_processes": len(all_processes),
                "security_tools": unique_tools,
                "top_processes": [{"name": n, "count": c} for n, c in top_processes],
                "sources": sources,
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
