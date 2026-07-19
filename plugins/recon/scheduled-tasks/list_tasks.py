#!/usr/bin/env python3
import sys, os, re
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

SUSPICIOUS_KEYWORDS = [
    "powershell", "cmd.exe", "certutil", "bitsadmin", "mshta", "wscript", "cscript",
    "regsvr32", "rundll32", "installutil", "msbuild", "regasm", "msiexec",
    "encoded", "bypass", "hidden", "-enc", "frombase64", "downloadstring",
    "invoke-expression", "iex(", "invoke-webrequest", "wget", "curl",
]
SYSTEM_PREFIXES = [
    "microsoft", "windows", "ms-", "google", "adobe", "apple", "mozilla",
    "oracle", "vmware", "symantec", "mcafee", "nvidia", "intel",
]


def parse_schtasks(text):
    tasks = []
    current = {}
    for line in text.splitlines():
        line = line.strip()
        if not line:
            continue
        m = re.match(r"^(TaskName)\s*:\s*(.+)", line, re.IGNORECASE)
        if m:
            if current.get("name"):
                tasks.append(current)
            current = {"name": m.group(2).strip(), "status": "", "trigger": "", "command": "", "author": "", "run_as": ""}
            continue
        for field, key in [("Status", "status"), ("Task To Run", "command"), ("Author", "author"),
                           ("Run As User", "run_as"), ("Schedule Type", "trigger"),
                           ("Start In", "start_in"), ("Scheduled Task State", "status")]:
            if line.startswith(field + ":") or line.startswith(field + " :"):
                val = line.split(":", 1)[1].strip()
                if key in current:
                    current[key] = val
    if current.get("name"):
        tasks.append(current)
    return tasks


def parse_crontab(text, user="root"):
    tasks = []
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split(None, 5)
        if len(parts) >= 6:
            schedule = " ".join(parts[:5])
            command = parts[5]
            tasks.append({
                "name": f"cron:{user}:{command[:50]}",
                "status": "ready",
                "trigger": schedule,
                "command": command,
                "author": user,
                "run_as": user,
            })
    return tasks


def parse_systemd_timers(text):
    tasks = []
    for line in text.splitlines():
        parts = line.split(None, 5)
        if len(parts) >= 5 and not line.startswith("NEXT") and not line.startswith("---"):
            tasks.append({
                "name": parts[0] if len(parts) > 0 else "",
                "status": "ready",
                "trigger": parts[1] if len(parts) > 1 else "",
                "command": parts[4] if len(parts) > 4 else "",
                "author": "system",
                "run_as": "root",
            })
    return tasks


def is_suspicious(task):
    cmd = (task.get("command", "") + " " + task.get("name", "")).lower()
    for kw in SUSPICIOUS_KEYWORDS:
        if kw in cmd:
            return True
    return False


def classify_task(task):
    name = (task.get("name", "") + " " + task.get("author", "")).lower()
    for prefix in SYSTEM_PREFIXES:
        if prefix in name:
            return "system"
    if task.get("author", "").lower() in ("system", "local system", "nt authority\\system"):
        return "system"
    return "user"


def main():
    data = read_stdin()
    params = data.get("params", {})
    target_agent = params.get("agent_id", "")

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        agents = db.all_agents()
        if target_agent:
            agents = [a for a in agents if a["id"] == target_agent or a["id"][:8] == target_agent]

        agent_results = []
        total_tasks = 0
        total_suspicious = 0
        status_counts = {}

        for agent in agents:
            tasks = db.tasks_for_agent(agent["id"])
            all_tasks = []
            for t in tasks:
                result_text = t.get("result", "") or ""
                cmd = (t.get("command", "") or "").lower()

                if "schtasks" in cmd and "/query" in cmd:
                    all_tasks.extend(parse_schtasks(result_text))
                if "crontab" in cmd or "/etc/crontab" in cmd:
                    user = "root"
                    if "crontab -l -u" in cmd:
                        m = re.search(r"-u\s+(\S+)", cmd)
                        if m:
                            user = m.group(1)
                    all_tasks.extend(parse_crontab(result_text, user))
                if "systemctl list-timers" in cmd:
                    all_tasks.extend(parse_systemd_timers(result_text))

            for task in all_tasks:
                task["classification"] = classify_task(task)
                task["suspicious"] = is_suspicious(task)
                s = task.get("status", "unknown")
                status_counts[s] = status_counts.get(s, 0) + 1

            suspicious = [t for t in all_tasks if t["suspicious"]]
            total_tasks += len(all_tasks)
            total_suspicious += len(suspicious)

            agent_results.append({
                "id": agent["id"][:8],
                "hostname": agent.get("hostname", ""),
                "os": agent.get("os", ""),
                "task_count": len(all_tasks),
                "suspicious_count": len(suspicious),
                "tasks": all_tasks[:50],
            })

        output_lines = [
            f"Scanned {len(agents)} agents",
            f"{total_tasks} scheduled tasks found",
            f"{total_suspicious} suspicious tasks",
        ]
        if status_counts:
            top = sorted(status_counts.items(), key=lambda x: -x[1])[:5]
            output_lines.append("Status: " + ", ".join(f"{k}={v}" for k, v in top))

        write_result(
            True,
            output="\n".join(output_lines),
            data={
                "total_agents": len(agents),
                "total_tasks": total_tasks,
                "suspicious_tasks": total_suspicious,
                "by_status": status_counts,
                "agents": agent_results,
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
