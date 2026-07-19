#!/usr/bin/env python3
import sys, os, re
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

SYSTEM_SHARES = {"c$", "d$", "admin$", "ipc$", "sysvol", "netlogon", "print$", "f$", "g$", "programs"}
USER_SHARES = {"documents", "desktop", "downloads", "public", "music", "pictures", "videos"}
INTERESTING_KEYWORDS = {"backup", "data", "secret", "confidential", "finance", "accounting", "hr", "passwords", "keys"}

SHARE_RE = re.compile(r"(?i)\\\\[\w.\-]+\\[\w.\-]+")
LOCAL_PATH_RE = re.compile(r"(?i)^(?:[a-z]:|\\\\[\w.\-]+\\[\w.\-]+)\s+\w+")
UNICODE_PATH_RE = re.compile(r"(?i)\\\\[\w.\-]+\\[\w.\-]+")
UNC_PATH_RE = re.compile(r"\\\\[\w.\-]+\\[\w.\-]+")
DRIVE_RE = re.compile(r"(?i)^([A-Z]):\\\\?\s+(\S+)")
WMIC_SHARE_RE = re.compile(r"(?i)Name\s*=\s*(\S+).*Path\s*=\s*(.*)")
MOUNT_RE = re.compile(r"(?i)//(\S+)/(\S+)\s+on\s+(\S+)")
LINUX_MOUNT_RE = re.compile(r"(?:smbfs|cifs|nfs|fuse)\s+.*?on\s+/mnt/\S+|on\s+/mnt/\S+")
DRIVE_LETTER_RE = re.compile(r"(?i)^([A-Z]):\s+(\\[\w.\-]+\\[\w.\-]+)")

def classify_share(name):
    low = name.lower().rstrip("\\")
    if low in SYSTEM_SHARES or low.startswith(("c$", "d$", "admin$")):
        return "system"
    if low in USER_SHARES:
        return "user"
    return "custom"

def detect_access(text):
    t = text.lower()
    if any(kw in t for kw in ("full control", "read/write", "rw", "writable")):
        return "read-write"
    if any(kw in t for kw in ("read-only", "readonly", "read only", "rx")):
        return "read-only"
    if any(kw in t for kw in ("denied", "access denied", "no access", "denied access")):
        return "no-access"
    return "unknown"

def is_interesting(name):
    low = name.lower()
    return any(kw in low for kw in INTERESTING_KEYWORDS)

def parse_net_share(output):
    shares = []
    for line in output.splitlines():
        line = line.strip()
        if not line or line.startswith("Share name") or line.startswith("---") or line.startswith("Shared resources"):
            continue
        m = re.match(r"(\S+)\s+(.*?)\s*$", line)
        if m:
            name = m.group(1)
            remark = m.group(2).strip()
            shares.append({"name": name, "path": "", "type": classify_share(name), "access": "unknown", "remark": remark})
        else:
            low = line.lower()
            if "read" in low or "write" in low or "denied" in low:
                if shares:
                    shares[-1]["access"] = detect_access(line)
            if line.lower().startswith("path"):
                path_val = line.split(":", 1)[-1].strip()
                if shares:
                    shares[-1]["path"] = path_val
    return shares

def parse_wmic_share(output):
    shares = []
    for line in output.splitlines():
        line = line.strip()
        if not line or line.startswith("AccessMask") or line.startswith("---"):
            continue
        m = WMIC_SHARE_RE.search(line)
        if m:
            name, path = m.group(1), m.group(2).strip()
            shares.append({"name": name, "path": path, "type": classify_share(name), "access": "unknown", "remark": ""})
        elif shares and shares[-1]["access"] == "unknown":
            shares[-1]["access"] = detect_access(line)
    return shares

def parse_net_use(output):
    shares = []
    for line in output.splitlines():
        m = re.search(r"(\\\\[\w.\-]+\\[\w.\-]+)", line)
        if m:
            unc = m.group(1)
            name = unc.split("\\")[-1]
            shares.append({"name": name, "path": unc, "type": classify_share(name), "access": "unknown", "remark": ""})
    return shares

def parse_wmic_disk(output):
    shares = []
    for line in output.splitlines():
        m = DRIVE_LETTER_RE.search(line)
        if m:
            letter, unc = m.group(1), m.group(2)
            shares.append({"name": letter, "path": unc, "type": classify_share(letter), "access": "unknown", "remark": ""})
    return shares

def parse_smbclient(output):
    shares = []
    in_list = False
    for line in output.splitlines():
        if "Sharename" in line and "Type" in line:
            in_list = True
            continue
        if in_list and line.startswith("---------"):
            continue
        if in_list and line.strip():
            parts = line.split(None, 1)
            if parts:
                name = parts[0]
                rest = parts[1] if len(parts) > 1 else ""
                shares.append({"name": name, "path": "", "type": classify_share(name), "access": detect_access(rest), "remark": rest.strip()})
        elif in_list and not line.strip():
            in_list = False
    return shares

def parse_linux_mounts(output):
    shares = []
    for line in output.splitlines():
        m = MOUNT_RE.search(line)
        if m:
            host, share_name, mount = m.group(1), m.group(2), m.group(3)
            shares.append({"name": f"{host}/{share_name}", "path": mount, "type": "custom", "access": "unknown", "remark": f"Mounted at {mount}"})
    return shares

def parse_ls_mnt(output):
    shares = []
    for line in output.splitlines():
        entry = line.strip()
        if not entry:
            continue
        shares.append({"name": entry, "path": f"/mnt/{entry}", "type": "custom", "access": "unknown", "remark": ""})
    return shares

def scan_agent_shares(agent, task_results):
    all_shares = []
    for result in task_results:
        output = result.get("output", "") or result.get("result", "") or ""
        task_type = (result.get("task_type", "") or result.get("type", "") or "").lower()
        if not output:
            continue
        if "net share" in task_type or "share" in task_type:
            all_shares.extend(parse_net_share(output))
        if "wmic" in task_type and "share" in task_type:
            all_shares.extend(parse_wmic_share(output))
        if "smbclient" in task_type:
            all_shares.extend(parse_smbclient(output))
        if "net use" in task_type:
            all_shares.extend(parse_net_use(output))
        if "wmic" in task_type and "logicaldisk" in task_type:
            all_shares.extend(parse_wmic_disk(output))
        if "mount" in task_type or "ls" in task_type:
            all_shares.extend(parse_linux_mounts(output))
            all_shares.extend(parse_ls_mnt(output))
        shares_found = SHARE_RE.findall(output)
        for s in shares_found:
            name = s.rstrip("\\").split("\\")[-1]
            if name and name not in {sh["name"].lower() for sh in all_shares}:
                all_shares.append({"name": name, "path": s, "type": classify_share(name), "access": "unknown", "remark": ""})
    if not all_shares:
        for result in task_results:
            output = result.get("output", "") or result.get("result", "") or ""
            for s in SHARE_RE.findall(output):
                name = s.rstrip("\\").split("\\")[-1]
                if name and name.lower() not in {sh["name"].lower() for sh in all_shares}:
                    all_shares.append({"name": name, "path": s, "type": classify_share(name), "access": "unknown", "remark": ""})
    seen = set()
    deduped = []
    for s in all_shares:
        key = (s["name"].lower(), s.get("path", "").lower())
        if key not in seen:
            seen.add(key)
            deduped.append(s)
    return deduped

def main():
    data = read_stdin()
    db = Database()
    agent_filter = data.get("params", {}).get("agent_id", "") if isinstance(data.get("params"), dict) else ""
    agents = db.get_agents()
    if agent_filter:
        agents = [a for a in agents if a.get("id") == agent_filter or a.get("hostname") == agent_filter]
    agent_shares_list = []
    total_shares = 0
    readable = 0
    writable = 0
    by_type = {"system": 0, "user": 0, "custom": 0}
    interesting = []
    for agent in agents:
        agent_id = agent.get("id", "")
        hostname = agent.get("hostname", "unknown")
        os_type = agent.get("os", agent.get("platform", "unknown"))
        task_results = db.get_task_results(agent_id)
        shares = scan_agent_shares(agent, task_results)
        agent_entry = {"id": agent_id, "hostname": hostname, "os": os_type, "shares": shares}
        agent_shares_list.append(agent_entry)
        for s in shares:
            total_shares += 1
            by_type[s["type"]] = by_type.get(s["type"], 0) + 1
            if s["access"] in ("read-only", "read-write", "unknown"):
                readable += 1
            if s["access"] == "read-write":
                writable += 1
            if is_interesting(s["name"]):
                interesting.append({"agent": hostname, "name": s["name"], "path": s.get("path", ""), "type": s["type"], "access": s["access"]})
    summary = {
        "total_agents": len(agents),
        "total_shares": total_shares,
        "by_type": by_type,
        "readable_shares": readable,
        "writable_shares": writable,
    }
    output = f"Scanned {len(agents)} agents | {total_shares} shares found | {readable} readable | {writable} writable"
    result_data = {"agents": agent_shares_list, "summary": summary, "interesting_shares": interesting}
    write_result(True, output=output, data=result_data)

if __name__ == "__main__":
    main()
