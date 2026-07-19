#!/usr/bin/env python3
"""Asset Inventory report plugin — generates a categorized asset inventory."""

import json
import os
import sys
from datetime import datetime

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_report


def generate_html(groups: dict, title: str, total: int, group_by: str) -> str:
    html = f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>{title}</title>
<style>
body {{ font-family: -apple-system, sans-serif; max-width: 900px; margin: 0 auto; padding: 20px; color: #1a1a2e; }}
h1 {{ color: #0f3460; border-bottom: 3px solid #e94560; padding-bottom: 10px; }}
h2 {{ color: #16213e; margin-top: 25px; }}
table {{ width: 100%; border-collapse: collapse; margin: 10px 0; }}
th, td {{ border: 1px solid #ddd; padding: 6px 10px; text-align: left; font-size: 12px; }}
th {{ background: #0f3460; color: white; }}
tr:nth-child(even) {{ background: #f8f9fa; }}
.badge {{ padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: bold; }}
.online {{ background: #d4edda; color: #155724; }}
.offline {{ background: #f8d7da; color: #721c24; }}
.section-count {{ color: #e94560; font-weight: bold; }}
</style></head><body>
<h1>{title}</h1>
<p>Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')} | Total Assets: {total} | Grouped by: {group_by}</p>
"""
    for group_name, agents in sorted(groups.items()):
        html += f"<h2>{group_name or 'Unknown'} <span class='section-count'>({len(agents)})</span></h2>"
        html += "<table><tr><th>ID</th><th>Hostname</th><th>User</th><th>IP</th><th>Public IP</th><th>Status</th><th>Integrity</th><th>Domain</th><th>Last Seen</th></tr>"
        for a in agents:
            sc = "online" if a["status"] == "online" else "offline"
            html += f"""<tr>
<td><code>{a['id'][:8]}</code></td><td>{a['hostname']}</td><td>{a['username']}</td>
<td>{a['ip']}</td><td>{a['public_ip']}</td>
<td><span class="badge {sc}">{a['status']}</span></td>
<td>{a['integrity']}</td><td>{a['domain']}</td><td>{a['last_seen']}</td></tr>"""
        html += "</table>"
    html += "</body></html>"
    return html


def main():
    data_input = read_stdin()
    params = data_input.get("params", {})
    config = data_input.get("config", {})

    fmt = params.get("format", "html")
    group_by = params.get("group_by", "os")
    title = params.get("title", "Asset Inventory")
    show_offline = config.get("show_offline", "true").lower() == "true"

    try:
        db = Database()
    except FileNotFoundError as e:
        print(json.dumps({"title": title, "format": fmt, "content": f"Error: {e}"}))
        return

    try:
        agents = db.all_agents()
        if not show_offline:
            agents = [a for a in agents if a.get("status") == "online"]

        groups = {}
        for a in agents:
            key = a.get(group_by) or "Unknown"
            if key not in groups:
                groups[key] = []
            groups[key].append({
                "id": a.get("id"),
                "hostname": a.get("hostname"),
                "username": a.get("username"),
                "os": a.get("os"),
                "ip": a.get("ip"),
                "public_ip": a.get("public_ip"),
                "status": a.get("status"),
                "integrity": a.get("integrity"),
                "domain": a.get("domain"),
                "country": a.get("country"),
                "last_seen": str(a.get("last_seen")),
            })

        if fmt == "json":
            content = json.dumps({"group_by": group_by, "groups": groups}, indent=2, default=str)
        elif fmt == "markdown":
            lines = [f"# {title}", f"*Grouped by: {group_by} | Total: {len(agents)}*", ""]
            for g, items in sorted(groups.items()):
                lines.append(f"## {g or 'Unknown'} ({len(items)})")
                lines.append("| ID | Hostname | OS | IP | Status |")
                lines.append("|---|---|---|---|---|")
                for a in items:
                    lines.append(f"| {a['id'][:8]} | {a['hostname']} | {a['os']} | {a['ip']} | {a['status']} |")
                lines.append("")
            content = "\n".join(lines)
        else:
            content = generate_html(groups, title, len(agents), group_by)
            fmt = "html"

        write_report(title, content, fmt)
    finally:
        db.close()


if __name__ == "__main__":
    main()
