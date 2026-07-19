#!/usr/bin/env python3
"""Engagement Report plugin — generates a comprehensive HTML/Markdown/JSON engagement summary."""

import json
import os
import sys
from datetime import datetime

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_report


def generate_html(data: dict, title: str) -> str:
    agents = data["agents"]
    tasks = data["tasks"]
    creds = data["creds"]
    listeners = data["listeners"]
    audit = data["audit"]

    status_counts = data["status_counts"]
    os_counts = data["os_counts"]
    task_status = data["task_status"]
    task_types = data["task_types"]
    cred_types = data["cred_types"]

    html = f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>{title}</title>
<style>
body {{ font-family: -apple-system, sans-serif; max-width: 900px; margin: 0 auto; padding: 20px; color: #1a1a2e; }}
h1 {{ color: #0f3460; border-bottom: 3px solid #e94560; padding-bottom: 10px; }}
h2 {{ color: #16213e; margin-top: 30px; }}
table {{ width: 100%; border-collapse: collapse; margin: 15px 0; }}
th, td {{ border: 1px solid #ddd; padding: 8px 12px; text-align: left; font-size: 13px; }}
th {{ background: #0f3460; color: white; }}
tr:nth-child(even) {{ background: #f8f9fa; }}
.stat {{ display: inline-block; background: #e8f4f8; border-left: 4px solid #0f3460; padding: 10px 15px; margin: 5px; border-radius: 4px; }}
.stat .num {{ font-size: 24px; font-weight: bold; color: #e94560; }}
.stat .label {{ font-size: 12px; color: #666; }}
.badge {{ display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: bold; }}
.online {{ background: #d4edda; color: #155724; }}
.offline {{ background: #f8d7da; color: #721c24; }}
.warn {{ background: #fff3cd; color: #856404; }}
</style></head><body>
<h1>{title}</h1>
<p>Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}</p>

<h2>Summary</h2>
<div>
  <div class="stat"><div class="num">{len(agents)}</div><div class="label">Total Agents</div></div>
  <div class="stat"><div class="num">{status_counts.get('online', 0)}</div><div class="label">Online</div></div>
  <div class="stat"><div class="num">{len(tasks)}</div><div class="label">Tasks Executed</div></div>
  <div class="stat"><div class="num">{len(creds)}</div><div class="label">Credentials</div></div>
  <div class="stat"><div class="num">{len(listeners)}</div><div class="label">Listeners</div></div>
</div>

<h2>Agent Inventory</h2>
<table><tr><th>ID</th><th>Hostname</th><th>User</th><th>OS</th><th>IP</th><th>Status</th><th>Integrity</th></tr>"""
    for a in agents[:50]:
        status_cls = "online" if a["status"] == "online" else "offline"
        html += f"""<tr>
<td><code>{a['id'][:8]}</code></td><td>{a['hostname']}</td><td>{a['username']}</td>
<td>{a['os']}</td><td>{a['ip']}</td>
<td><span class="badge {status_cls}">{a['status']}</span></td>
<td>{a['integrity']}</td></tr>"""
    html += "</table>"

    html += f"""<h2>Task Statistics</h2>
<table><tr><th>Metric</th><th>Value</th></tr>"""
    for k, v in task_status.items():
        html += f"<tr><td>{k}</td><td>{v}</td></tr>"
    html += "</table>"

    html += f"""<h2>Task Types</h2>
<table><tr><th>Type</th><th>Count</th></tr>"""
    for k, v in sorted(task_types.items(), key=lambda x: -x[1]):
        html += f"<tr><td>{k}</td><td>{v}</td></tr>"
    html += "</table>"

    if creds:
        html += f"""<h2>Credentials</h2>
<table><tr><th>Type</th><th>Count</th></tr>"""
        for k, v in sorted(cred_types.items(), key=lambda x: -x[1]):
            html += f"<tr><td>{k}</td><td>{v}</td></tr>"
        html += "</table>"

    if audit:
        html += f"""<h2>Recent Activity</h2>
<table><tr><th>Time</th><th>User</th><th>Action</th><th>IP</th></tr>"""
        for a in audit[:20]:
            html += f"<tr><td>{a['created_at']}</td><td>{a['user']}</td><td>{a['action']}</td><td>{a['ip']}</td></tr>"
        html += "</table>"

    html += "</body></html>"
    return html


def generate_markdown(data: dict, title: str) -> str:
    agents = data["agents"]
    tasks = data["tasks"]
    creds = data["creds"]
    lines = [
        f"# {title}",
        f"*Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}*",
        "",
        "## Summary",
        f"- **Total Agents:** {len(agents)}",
        f"- **Online:** {data['status_counts'].get('online', 0)}",
        f"- **Tasks:** {len(tasks)}",
        f"- **Credentials:** {len(creds)}",
        f"- **Listeners:** {len(data['listeners'])}",
        "",
        "## Agent Inventory",
        "| ID | Hostname | OS | IP | Status |",
        "|---|---|---|---|---|",
    ]
    for a in agents[:30]:
        lines.append(f"| {a['id'][:8]} | {a['hostname']} | {a['os']} | {a['ip']} | {a['status']} |")

    lines += ["", "## Task Types", "| Type | Count |", "|---|---|"]
    for k, v in sorted(data["task_types"].items(), key=lambda x: -x[1]):
        lines.append(f"| {k} | {v} |")

    return "\n".join(lines)


def main():
    data_input = read_stdin()
    params = data_input.get("params", {})
    config = data_input.get("config", {})

    fmt = params.get("format", "html")
    title = params.get("title", "Engagement Report")

    try:
        db = Database()
    except FileNotFoundError as e:
        print(json.dumps({"title": title, "format": fmt, "content": f"Error: {e}"}))
        return

    try:
        agents = db.all_agents()
        tasks = db.all_tasks()
        creds = db.all_credentials()
        listeners = db.all_listeners()
        audit = db.all_audit_logs()[:50]

        status_counts = {}
        for a in agents:
            s = a.get("status", "unknown")
            status_counts[s] = status_counts.get(s, 0) + 1

        os_counts = {}
        for a in agents:
            o = a.get("os", "unknown") or "unknown"
            os_counts[o] = os_counts.get(o, 0) + 1

        task_status = {}
        for t in tasks:
            s = t.get("status", "unknown")
            task_status[s] = task_status.get(s, 0) + 1

        task_types = {}
        for t in tasks:
            tp = t.get("type", "unknown")
            task_types[tp] = task_types.get(tp, 0) + 1

        cred_types = {}
        for c in creds:
            ct = c.get("type") or "unknown"
            cred_types[ct] = cred_types.get(ct, 0) + 1

        report_data = {
            "agents": agents,
            "tasks": tasks,
            "creds": creds,
            "listeners": listeners,
            "audit": audit,
            "status_counts": status_counts,
            "os_counts": os_counts,
            "task_status": task_status,
            "task_types": task_types,
            "cred_types": cred_types,
        }

        if fmt == "json":
            content = json.dumps(report_data, indent=2, default=str)
        elif fmt == "markdown":
            content = generate_markdown(report_data, title)
        else:
            content = generate_html(report_data, title)
            fmt = "html"

        write_report(title, content, fmt)
    finally:
        db.close()


if __name__ == "__main__":
    main()
