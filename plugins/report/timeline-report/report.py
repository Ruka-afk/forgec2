#!/usr/bin/env python3
"""Timeline Report plugin — generates a chronological event timeline."""

import json
import os
import sys
from datetime import datetime, timedelta

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_report

EVENT_COLORS = {
    "agent_connect": "#28a745",
    "agent_disconnect": "#dc3545",
    "task_created": "#007bff",
    "task_completed": "#17a2b8",
    "task_failed": "#dc3545",
    "user_login": "#6f42c1",
    "user_logout": "#6c757d",
    "credential": "#fd7e14",
    "audit": "#343a40",
}


def generate_html(events: list, title: str) -> str:
    html = f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>{title}</title>
<style>
body {{ font-family: -apple-system, sans-serif; max-width: 900px; margin: 0 auto; padding: 20px; color: #1a1a2e; }}
h1 {{ color: #0f3460; border-bottom: 3px solid #e94560; padding-bottom: 10px; }}
.timeline {{ position: relative; padding: 20px 0; }}
.timeline::before {{ content: ''; position: absolute; left: 20px; top: 0; bottom: 0; width: 3px; background: #ddd; }}
.event {{ position: relative; margin: 10px 0 10px 45px; padding: 10px 15px; border-radius: 8px; border-left: 4px solid #ccc; background: #f8f9fa; }}
.event::before {{ content: ''; position: absolute; left: -33px; top: 15px; width: 12px; height: 12px; border-radius: 50%; border: 2px solid white; }}
.event .time {{ font-size: 11px; color: #666; }}
.event .type {{ font-weight: bold; font-size: 13px; }}
.event .detail {{ font-size: 12px; color: #444; margin-top: 4px; }}
.event .agent {{ font-size: 11px; color: #888; }}
</style></head><body>
<h1>{title}</h1>
<p>Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')} | Events: {len(events)}</p>
<div class="timeline">
"""
    for ev in events:
        color = EVENT_COLORS.get(ev["category"], "#6c757d")
        html += f"""<div class="event" style="border-left-color:{color}">
<div class="event" style="border-left-color:{color}">
<div class="time">{ev['timestamp']}</div>
<div class="type" style="color:{color}">{ev['type']}</div>
<div class="detail">{ev['detail']}</div>
<div class="agent">{ev.get('agent_id', '')} {ev.get('user', '')}</div>
</div>
"""
    html += "</div></body></html>"
    return html


def main():
    data_input = read_stdin()
    params = data_input.get("params", {})
    config = data_input.get("config", {})

    fmt = params.get("format", "html")
    title = params.get("title", "Engagement Timeline")
    hours = int(params.get("hours", 72))
    max_events = int(config.get("max_events", 500))

    try:
        db = Database()
    except FileNotFoundError as e:
        print(json.dumps({"title": title, "format": fmt, "content": f"Error: {e}"}))
        return

    try:
        events = []

        # Audit logs
        audit_logs = db.all_audit_logs()
        for a in audit_logs:
            ts = str(a.get("created_at", ""))
            if hours > 0:
                try:
                    dt = datetime.fromisoformat(ts.replace("Z", "+00:00"))
                    if dt < datetime.now(dt.tzinfo) - timedelta(hours=hours):
                        continue
                except Exception:
                    pass
            action = a.get("action", "")
            category = "audit"
            if "login" in action:
                category = "user_login" if "out" not in action else "user_logout"
            events.append({
                "timestamp": ts,
                "type": action,
                "category": category,
                "detail": f"{a.get('action')} on {a.get('resource', 'N/A')}",
                "user": a.get("user", ""),
                "agent_id": a.get("agent_id", ""),
            })

        # Tasks (created and completed)
        tasks = db.all_tasks()
        for t in tasks:
            ts = str(t.get("created_at", ""))
            if hours > 0:
                try:
                    dt = datetime.fromisoformat(ts.replace("Z", "+00:00"))
                    if dt < datetime.now(dt.tzinfo) - timedelta(hours=hours):
                        continue
                except Exception:
                    pass
            events.append({
                "timestamp": ts,
                "type": f"task:{t.get('type', '?')}",
                "category": "task_created",
                "detail": f"Task #{t.get('id')}: {(t.get('command') or '')[:80]}",
                "agent_id": t.get("agent_id", ""),
                "user": t.get("created_by", ""),
            })
            if t.get("status") in ("completed", "failed"):
                completed_at = str(t.get("updated_at", ""))
                cat = "task_completed" if t["status"] == "completed" else "task_failed"
                events.append({
                    "timestamp": completed_at,
                    "type": f"task:{t['status']}",
                    "category": cat,
                    "detail": f"Task #{t.get('id')} {t['status']}" + (f": {t.get('error', '')[:60]}" if t.get("error") else ""),
                    "agent_id": t.get("agent_id", ""),
                })

        # Sort by timestamp
        events.sort(key=lambda e: e["timestamp"], reverse=True)
        events = events[:max_events]

        if fmt == "json":
            content = json.dumps(events, indent=2, default=str)
        elif fmt == "markdown":
            lines = [f"# {title}", f"*Events: {len(events)}*", "", "| Time | Type | Detail | User |", "|---|---|---|---|"]
            for ev in events:
                lines.append(f"| {ev['timestamp'][:19]} | {ev['type']} | {ev['detail'][:60]} | {ev.get('user', '')} |")
            content = "\n".join(lines)
        else:
            content = generate_html(events, title)
            fmt = "html"

        write_report(title, content, fmt)
    finally:
        db.close()


if __name__ == "__main__":
    main()
