#!/usr/bin/env python3
import json, os, sys
from datetime import datetime, timedelta
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_report


def _parse_ts(ts_str):
    if not ts_str:
        return None
    for fmt in ("%Y-%m-%dT%H:%M:%S", "%Y-%m-%d %H:%M:%S", "%Y-%m-%dT%H:%M:%S.%f"):
        try:
            return datetime.strptime(ts_str[:26], fmt)
        except (ValueError, IndexError):
            continue
    return None


def _event_type_color(etype):
    if etype in ("task.completed", "success"):
        return "#28a745"
    if etype in ("task.failed", "error"):
        return "#dc3545"
    if etype in ("task.created", "task.pending"):
        return "#007bff"
    if etype in ("agent.offline", "agent.disconnect"):
        return "#fd7e14"
    if etype in ("agent.online", "agent.connect"):
        return "#17a2b8"
    if etype in ("audit",):
        return "#6f42c1"
    return "#6c757d"


def generate_html(data, title):
    events = data["events"]
    summary = data["summary"]

    html = f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>{title}</title>
<style>
body {{ font-family: -apple-system, sans-serif; max-width: 900px; margin: 0 auto; padding: 20px; color: #1a1a2e; }}
h1 {{ color: #0f3460; border-bottom: 3px solid #e94560; padding-bottom: 10px; }}
h2 {{ color: #16213e; margin-top: 30px; }}
.stat {{ display: inline-block; background: #e8f4f8; border-left: 4px solid #0f3460; padding: 10px 15px; margin: 5px; border-radius: 4px; }}
.stat .num {{ font-size: 24px; font-weight: bold; color: #e94560; }}
.stat .label {{ font-size: 12px; color: #666; }}
.timeline {{ position: relative; padding-left: 30px; margin-top: 20px; }}
.timeline::before {{ content: ''; position: absolute; left: 12px; top: 0; bottom: 0; width: 2px; background: #ddd; }}
.event {{ position: relative; margin-bottom: 16px; padding: 10px 14px; background: #f8f9fa; border-radius: 6px; border-left: 4px solid #6c757d; }}
.event::before {{ content: ''; position: absolute; left: -26px; top: 14px; width: 12px; height: 12px; border-radius: 50%; background: #6c757d; border: 2px solid white; }}
.event .time {{ font-size: 11px; color: #888; }}
.event .type {{ font-weight: bold; font-size: 13px; }}
.event .detail {{ font-size: 12px; color: #555; margin-top: 4px; }}
.day-header {{ background: #0f3460; color: white; padding: 6px 12px; border-radius: 4px; margin: 20px 0 10px; font-weight: bold; font-size: 13px; }}
</style></head><body>
<h1>{title}</h1>
<p>Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}</p>

<h2>Summary</h2>
<div>
  <div class="stat"><div class="num">{summary['total_events']}</div><div class="label">Total Events</div></div>
  <div class="stat"><div class="num">{summary['peak_hour']}</div><div class="label">Peak Hour</div></div>
  <div class="stat"><div class="num">{summary['active_agents']}</div><div class="label">Active Agents</div></div>
  <div class="stat"><div class="num">{summary['tasks_completed']}</div><div class="label">Tasks Completed</div></div>
  <div class="stat"><div class="num">{summary['tasks_failed']}</div><div class="label">Tasks Failed</div></div>
</div>

<h2>Timeline</h2>
<div class="timeline">"""

    current_day = ""
    for ev in events:
        ts = _parse_ts(ev.get("timestamp", ""))
        day = ts.strftime("%Y-%m-%d") if ts else "Unknown"
        if day != current_day:
            current_day = day
            html += f'<div class="day-header">{day}</div>'

        color = _event_type_color(ev.get("event_type", ""))
        time_str = ts.strftime("%H:%M:%S") if ts else "??:??:??"
        etype = ev.get("event_type", "unknown")
        detail = ev.get("detail", "")
        agent = ev.get("agent_id", "")[:8]

        html += f"""<div class="event" style="border-left-color:{color}">
<div class="event::before" style="background:{color}"></div>
<div class="time">{time_str}</div>
<div class="type" style="color:{color}">{etype}</div>
<div class="detail">{detail} {'(' + agent + ')' if agent else ''}</div>
</div>"""

    html += "</div></body></html>"
    return html


def generate_markdown(data, title):
    summary = data["summary"]
    events = data["events"]
    lines = [
        f"# {title}",
        f"*Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}*",
        "",
        "## Summary",
        f"- **Total Events:** {summary['total_events']}",
        f"- **Peak Hour:** {summary['peak_hour']}",
        f"- **Active Agents:** {summary['active_agents']}",
        f"- **Tasks Completed:** {summary['tasks_completed']}",
        f"- **Tasks Failed:** {summary['tasks_failed']}",
        "",
        "## Timeline",
        "| Time | Event | Agent | Detail |",
        "|---|---|---|---|",
    ]
    for ev in events[:100]:
        ts = _parse_ts(ev.get("timestamp", ""))
        time_str = ts.strftime("%H:%M:%S") if ts else "??:??:??"
        lines.append(f"| {time_str} | {ev.get('event_type','')} | {ev.get('agent_id','')[:8]} | {ev.get('detail','')} |")
    return "\n".join(lines)


def main():
    data_input = read_stdin()
    params = data_input.get("params", {})
    config = data_input.get("config", {})
    fmt = params.get("format", "html")
    title = params.get("title", "Operational Timeline")
    hours = int(params.get("hours", 0))
    target_agent = params.get("agent_id", "")

    try:
        db = Database()
    except FileNotFoundError as e:
        print(json.dumps({"title": title, "format": fmt, "content": f"Error: {e}"}))
        return

    try:
        events = []
        tasks = db.all_tasks()
        agents = db.all_agents()
        audit = db.all_audit_logs()

        cutoff = None
        if hours > 0:
            cutoff = datetime.utcnow() - timedelta(hours=hours)

        for t in tasks:
            ts = _parse_ts(t.get("created_at", ""))
            if cutoff and ts and ts < cutoff:
                continue
            if target_agent and t.get("agent_id", "") != target_agent and t.get("agent_id", "")[:8] != target_agent:
                continue
            status = t.get("status", "pending")
            events.append({
                "timestamp": t.get("created_at", ""),
                "event_type": f"task.{status}",
                "agent_id": t.get("agent_id", ""),
                "detail": f"{t.get('type', '')}: {(t.get('command', '') or '')[:80]}",
            })

        for a in agents:
            ls = a.get("last_seen", "")
            ts = _parse_ts(ls)
            if cutoff and ts and ts < cutoff:
                continue
            if target_agent and a["id"] != target_agent and a["id"][:8] != target_agent:
                continue
            status = a.get("status", "unknown")
            events.append({
                "timestamp": ls,
                "event_type": f"agent.{'online' if status == 'online' else 'offline'}",
                "agent_id": a["id"],
                "detail": f"{a.get('hostname', '')} ({a.get('ip', '')}) [{status}]",
            })

        for log in audit:
            ts = _parse_ts(log.get("created_at", ""))
            if cutoff and ts and ts < cutoff:
                continue
            events.append({
                "timestamp": log.get("created_at", ""),
                "event_type": "audit",
                "agent_id": "",
                "detail": f"{log.get('user', '')} {log.get('action', '')} from {log.get('ip', '')}",
            })

        events.sort(key=lambda e: e.get("timestamp", ""), reverse=True)

        hour_counts = {}
        task_completed = 0
        task_failed = 0
        active_agents = set()
        for ev in events:
            ts = _parse_ts(ev.get("timestamp", ""))
            if ts:
                hour_counts[ts.hour] = hour_counts.get(ts.hour, 0) + 1
            if ev["event_type"] == "task.completed":
                task_completed += 1
            elif ev["event_type"] == "task.failed":
                task_failed += 1
            if ev.get("agent_id"):
                active_agents.add(ev["agent_id"][:8])

        peak_hour = max(hour_counts, key=hour_counts.get) if hour_counts else "N/A"
        if isinstance(peak_hour, int):
            peak_hour = f"{peak_hour:02d}:00"

        summary = {
            "total_events": len(events),
            "peak_hour": peak_hour,
            "active_agents": len(active_agents),
            "tasks_completed": task_completed,
            "tasks_failed": task_failed,
        }

        report_data = {"events": events, "summary": summary}

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
