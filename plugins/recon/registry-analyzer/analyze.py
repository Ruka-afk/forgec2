#!/usr/bin/env python3
import sys, os, re
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

AUTO_START_KEYS = [
    r"HKLM\Software\Microsoft\Windows\CurrentVersion\Run",
    r"HKLM\Software\Microsoft\Windows\CurrentVersion\RunOnce",
    r"HKCU\Software\Microsoft\Windows\CurrentVersion\Run",
    r"HKLM\Software\Microsoft\Windows\CurrentVersion\RunServices",
    r"HKLM\Software\Microsoft\Windows NT\CurrentVersion\Winlogon",
    r"HKLM\System\CurrentControlSet\Services",
    r"HKLM\Software\Microsoft\Windows\CurrentVersion\Explorer\Shell Folders",
    r"HKLM\Software\Microsoft\Windows\CurrentVersion\Explorer\Browser Helper Objects",
]

INTERESTING_VALUES = {
    "DisableRegistryTools": {"risk": "high", "reason": "Anti-forensics: disables registry editor"},
    "EnableLUA": {"risk": "medium", "reason": "UAC bypass indicator"},
    "SecurityHealth": {"risk": "low", "reason": "Windows Defender status setting"},
    "CommandLine": {"risk": "high", "reason": "Service hijacking: arbitrary command in service entry"},
}

MICROSOFT_PATTERNS = (
    "microsoft", "windows", "ms", "system32\\ms", "syswow64\\ms",
    "program files\\common files\\microsoft", "\\windows\\system32\\",
    "\\windows\\syswow64\\", "\\windows\\system\\", "rundll32.exe shell32.dll",
    "rundll32.exe \"C:\\Windows",
)


def parse_reg_query(text):
    entries = []
    current_key = None
    for line in text.splitlines():
        line = line.strip()
        if not line:
            continue
        key_match = re.match(r"^(HK[^ ]+)$", line)
        if key_match:
            current_key = key_match.group(1)
            continue
        if current_key and line.startswith(current_key):
            rest = line[len(current_key):].strip()
            parts = rest.split(None, 1)
            if len(parts) == 2:
                val_name = parts[0]
                val_data = parts[1]
            else:
                val_name = parts[0] if parts else ""
                val_data = ""
            entries.append({"key": current_key, "name": val_name, "data": val_data})
            continue
        value_match = re.match(r"^\s+(\S+)\s+(REG_\w+)\s+(.*)$", line)
        if value_match and current_key:
            entries.append({
                "key": current_key,
                "name": value_match.group(1),
                "data": value_match.group(3).strip(),
            })
    return entries


def classify_entry(data):
    lower = data.lower()
    for pat in MICROSOFT_PATTERNS:
        if pat in lower:
            return "system"
    if "\\windows\\" in lower or "rundll32" in lower:
        return "system"
    return "third-party"


def check_path_validity(data):
    path_patterns = [
        r"[A-Z]:\\[^\s]+",
        r"\\\\[^\s]+",
        r"%[^%]+%\\",
    ]
    for pat in path_patterns:
        if re.search(pat, data, re.IGNORECASE):
            return True
    return False


def scan_interesting_values(text):
    found = []
    for name, meta in INTERESTING_VALUES.items():
        pattern = re.compile(re.escape(name), re.IGNORECASE)
        for line in text.splitlines():
            if pattern.search(line):
                parts = line.strip().split(None, 2)
                val_data = parts[2] if len(parts) >= 3 else ""
                found.append({
                    "key": "",
                    "name": name,
                    "value": val_data.strip(),
                    "risk": meta["risk"],
                    "reason": meta["reason"],
                })
    return found


def main():
    data = read_stdin()
    agent_id = data.get("agent_id", "")

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        agents = [db.agent_by_id(agent_id)] if agent_id else db.all_agents()
        agents = [a for a in agents if a]

        all_agent_results = []
        total_entries = 0
        total_system = 0
        total_third = 0
        total_suspicious = 0

        for agent in agents:
            aid = agent.get("id", "")
            hostname = agent.get("hostname", agent.get("name", ""))
            os_type = (agent.get("os", "") or "").lower()
            if "windows" not in os_type and "win" not in os_type:
                continue

            tasks = db.tasks_for_agent(aid)
            reg_tasks = [
                t for t in tasks
                if t.get("status") == "completed"
                and any(
                    kw in (t.get("command", "") or "").lower()
                    for kw in ("reg query", "reg ", "hklm\\", "hkcu\\", "registry")
                )
            ]

            agent_entries = []
            agent_interesting = []

            for task in reg_tasks[:20]:
                result_text = task.get("result", "")
                if not result_text:
                    continue
                entries = parse_reg_query(result_text)
                for e in entries:
                    cls = classify_entry(e["data"])
                    valid_path = check_path_validity(e["data"])
                    agent_entries.append({
                        "key": e["key"],
                        "name": e["name"],
                        "data": e["data"],
                        "classification": cls,
                        "valid_path": valid_path,
                    })
                    if cls == "system":
                        total_system += 1
                    else:
                        total_third += 1
                total_entries += len(entries)

                interesting = scan_interesting_values(result_text)
                agent_interesting.extend(interesting)
                total_suspicious += len(interesting)

            all_agent_results.append({
                "id": aid,
                "hostname": hostname,
                "entries": agent_entries,
                "interesting_values": agent_interesting,
            })

        summary = {
            "total_agents": len(all_agent_results),
            "total_entries": total_entries,
            "system_entries": total_system,
            "third_party": total_third,
            "suspicious": total_suspicious,
        }

        output = (
            f"Scanned {len(all_agent_results)} agents | "
            f"{total_entries} registry entries | "
            f"{total_suspicious} suspicious"
        )

        write_result(True, output=output, data={"agents": all_agent_results, "summary": summary})
    finally:
        db.close()


if __name__ == "__main__":
    main()
