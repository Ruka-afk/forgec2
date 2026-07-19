#!/usr/bin/env python3
"""Real-Time Dashboard plugin — self-refreshing operational dashboard with auto-reload."""

import json
import os
import sys
from datetime import datetime, timedelta
from collections import defaultdict

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_report


def _parse_dt(s):
    if not s:
        return None
    try:
        return datetime.fromisoformat(s.replace("Z", "+00:00")).replace(tzinfo=None)
    except (ValueError, TypeError):
        return None


def collect_data(db):
    agents = db.all_agents()
    tasks = db.all_tasks()
    credentials = db.all_credentials()
    listeners = db.all_listeners()
    audit_logs = db.all_audit_logs()
    network_hosts = db.all_network_hosts()
    return agents, tasks, credentials, listeners, audit_logs, network_hosts


def compute_metrics(agents, tasks, credentials, listeners, audit_logs, network_hosts):
    now = datetime.utcnow()
    cutoff_24h = now - timedelta(hours=24)

    # Agent status counts
    status_counts = {"online": 0, "offline": 0, "stale": 0, "burned": 0}
    for a in agents:
        status = (a.get("status") or "unknown").lower()
        if status in status_counts:
            status_counts[status] += 1
        else:
            last_seen = _parse_dt(a.get("last_seen") or "")
            if last_seen and (now - last_seen) > timedelta(hours=24):
                status_counts["burned"] += 1
            else:
                status_counts["offline"] += 1

    # Task throughput: tasks per hour for last 24h
    hourly_tasks = defaultdict(int)
    recent_tasks = []
    for t in tasks:
        created = _parse_dt(t.get("created_at") or "")
        if created and created >= cutoff_24h:
            hour_key = created.strftime("%H:00")
            hourly_tasks[hour_key] += 1
        if created and created >= cutoff_24h:
            recent_tasks.append(t)

    # Last 24h task count
    tasks_24h = sum(hourly_tasks.values())
    tasks_per_hour = round(tasks_24h / 24, 1) if tasks_24h else 0

    # Recent events: last 20 tasks
    recent_tasks_sorted = sorted(recent_tasks, key=lambda t: t.get("created_at") or "", reverse=True)[:20]

    # Last 10 audit entries
    recent_audit = audit_logs[:10]

    # Credential collection rate (creds per day over last 7 days)
    cutoff_7d = now - timedelta(days=7)
    creds_7d = 0
    for c in credentials:
        created = _parse_dt(c.get("created_at") or "")
        if created and created >= cutoff_7d:
            creds_7d += 1
    creds_per_day = round(creds_7d / 7, 1)

    # Listener status
    listener_info = []
    for l in listeners:
        status = "active" if (l.get("enabled") or l.get("status") == "active") else "disabled"
        listener_info.append({
            "id": l.get("id", "?"),
            "name": l.get("name") or l.get("type") or "Unknown",
            "type": l.get("type") or "unknown",
            "status": status,
            "port": l.get("port") or l.get("bind_port") or "?",
            "created_at": l.get("created_at") or "",
        })

    # Geographic distribution
    geo_counts = defaultdict(int)
    for a in agents:
        country = a.get("country") or a.get("geo") or "Unknown"
        geo_counts[country] += 1

    # OS distribution
    os_counts = defaultdict(int)
    for a in agents:
        os_name = a.get("os") or "Unknown"
        os_counts[os_name] += 1

    # Network hosts
    hosts_count = len(network_hosts)

    return {
        "now": now,
        "total_agents": len(agents),
        "status_counts": status_counts,
        "total_tasks": len(tasks),
        "tasks_24h": tasks_24h,
        "tasks_per_hour": tasks_per_hour,
        "hourly_tasks": dict(hourly_tasks),
        "recent_tasks": recent_tasks_sorted,
        "recent_audit": recent_audit,
        "total_creds": len(credentials),
        "creds_per_day": creds_per_day,
        "listeners": listener_info,
        "total_listeners": len(listeners),
        "geo_counts": dict(geo_counts),
        "os_counts": dict(os_counts),
        "hosts_count": hosts_count,
    }


def generate_html(metrics, title, refresh_seconds):
    now = metrics["now"]
    sc = metrics["status_counts"]

    # Build task throughput chart (text-based bars)
    max_hour = max(metrics["hourly_tasks"].values()) if metrics["hourly_tasks"] else 1
    chart_bars = ""
    for h in range(24):
        hour_key = f"{h:02d}:00"
        count = metrics["hourly_tasks"].get(hour_key, 0)
        pct = (count / max_hour * 100) if max_hour else 0
        bar_color = "#e94560" if count > 0 else "#1a1a3e"
        chart_bars += f"""
        <div class="chart-bar-wrap">
          <div class="chart-bar" style="height:{max(pct, 2)}%;background:{bar_color};" title="{count} tasks"></div>
          <div class="chart-label">{h:02d}</div>
        </div>"""

    # Status indicator color helper
    def status_color(s):
        return {
            "online": "#00e676",
            "offline": "#ff5252",
            "stale": "#ffab40",
            "burned": "#e040fb",
            "active": "#00e676",
            "disabled": "#ff5252",
        }.get(s, "#607d8b")

    # Recent tasks rows
    task_rows = ""
    for t in metrics["recent_tasks"]:
        ts = t.get("created_at") or ""
        status = t.get("status") or "pending"
        task_rows += f"""
        <tr>
          <td class="mono">{t.get('id', '?')}</td>
          <td>{t.get('type', '?')}</td>
          <td><span class="status-dot" style="background:{status_color(status)}"></span>{status}</td>
          <td class="mono dim">{ts[:19]}</td>
        </tr>"""

    # Audit rows
    audit_rows = ""
    for a in metrics["recent_audit"]:
        ts = a.get("created_at") or ""
        audit_rows += f"""
        <tr>
          <td class="mono">{a.get('action', '?')}</td>
          <td>{a.get('resource', '?')}</td>
          <td>{a.get('user', '?')}</td>
          <td class="mono dim">{ts[:19]}</td>
        </tr>"""

    # Agent table rows
    agent_rows = ""
    # Build a fresh agent list from metrics to display status indicators
    # We need agents again — use status_counts breakdown
    # Actually we have access via metrics; we'll show a summary table instead
    for status_label in ["online", "offline", "stale", "burned"]:
        cnt = sc.get(status_label, 0)
        agent_rows += f"""
        <tr>
          <td><span class="status-dot" style="background:{status_color(status_label)}"></span>{status_label.capitalize()}</td>
          <td class="num">{cnt}</td>
          <td class="num">{round(cnt / metrics['total_agents'] * 100, 1) if metrics['total_agents'] else 0}%</td>
        </tr>"""

    # OS distribution
    os_rows = ""
    for os_name, cnt in sorted(metrics["os_counts"].items(), key=lambda x: -x[1]):
        os_rows += f"""
        <tr>
          <td>{os_name}</td>
          <td class="num">{cnt}</td>
        </tr>"""

    # Geo distribution
    geo_rows = ""
    for country, cnt in sorted(metrics["geo_counts"].items(), key=lambda x: -x[1]):
        geo_rows += f"""
        <tr>
          <td>{country}</td>
          <td class="num">{cnt}</td>
        </tr>"""

    # Listener rows
    listener_rows = ""
    for l in metrics["listeners"]:
        listener_rows += f"""
        <tr>
          <td class="mono">{l['id']}</td>
          <td>{l['name']}</td>
          <td>{l['type']}</td>
          <td>{l['port']}</td>
          <td><span class="status-dot" style="background:{status_color(l['status'])}"></span>{l['status']}</td>
        </tr>"""

    # Event feed items
    event_items = ""
    for t in metrics["recent_tasks"][:10]:
        status = t.get("status") or "pending"
        event_items += f"""
        <div class="event-item">
          <span class="status-dot" style="background:{status_color(status)}"></span>
          <span class="event-text">Task #{t.get('id','?')} [{t.get('type','?')}] &rarr; {status}</span>
          <span class="event-time">{(t.get('created_at') or '')[:16]}</span>
        </div>"""
    for a in metrics["recent_audit"][:5]:
        event_items += f"""
        <div class="event-item">
          <span class="status-dot" style="background:#607d8b"></span>
          <span class="event-text">Audit: {a.get('action','?')} on {a.get('resource','?')}</span>
          <span class="event-time">{(a.get('created_at') or '')[:16]}</span>
        </div>"""

    html = f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta http-equiv="refresh" content="{refresh_seconds}">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{title}</title>
<style>
  * {{ margin:0; padding:0; box-sizing:border-box; }}
  body {{
    font-family: 'Segoe UI', -apple-system, sans-serif;
    background: #0d1117;
    color: #c9d1d9;
    min-height: 100vh;
    animation: fadeIn 0.4s ease-in;
  }}
  @keyframes fadeIn {{ from {{ opacity:0.6; }} to {{ opacity:1; }} }}
  @keyframes pulse {{ 0%,100% {{ opacity:1; }} 50% {{ opacity:0.4; }} }}
  .container {{ max-width: 1280px; margin: 0 auto; padding: 20px; }}
  header {{
    display: flex; justify-content: space-between; align-items: center;
    border-bottom: 1px solid #21262d; padding-bottom: 16px; margin-bottom: 24px;
  }}
  h1 {{ font-size: 22px; color: #f0f6fc; font-weight: 600; }}
  .refresh-badge {{
    background: #161b22; border: 1px solid #30363d; border-radius: 16px;
    padding: 4px 14px; font-size: 12px; color: #8b949e;
    display: flex; align-items: center; gap: 6px;
  }}
  .refresh-dot {{
    width: 8px; height: 8px; border-radius: 50%; background: #00e676;
    animation: pulse 2s infinite;
  }}
  .subtitle {{ color: #8b949e; font-size: 13px; margin-top: 2px; }}

  /* Stat cards */
  .stat-grid {{
    display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 16px; margin-bottom: 28px;
  }}
  .stat-card {{
    background: #161b22; border: 1px solid #21262d; border-radius: 10px;
    padding: 20px; position: relative; overflow: hidden;
    transition: transform 0.15s, box-shadow 0.15s;
  }}
  .stat-card:hover {{ transform: translateY(-2px); box-shadow: 0 4px 16px rgba(0,0,0,0.3); }}
  .stat-card .accent {{ position: absolute; top: 0; left: 0; right: 0; height: 3px; }}
  .stat-card .num {{ font-size: 36px; font-weight: 700; color: #f0f6fc; line-height: 1.1; }}
  .stat-card .label {{ font-size: 12px; color: #8b949e; margin-top: 4px; text-transform: uppercase; letter-spacing: 0.5px; }}
  .stat-card .sub {{ font-size: 13px; color: #58a6ff; margin-top: 6px; }}

  /* Section */
  .section {{
    background: #161b22; border: 1px solid #21262d; border-radius: 10px;
    padding: 20px; margin-bottom: 20px;
  }}
  .section-title {{
    font-size: 14px; font-weight: 600; color: #f0f6fc; margin-bottom: 14px;
    display: flex; align-items: center; gap: 8px;
  }}
  .section-title::before {{
    content: ''; width: 4px; height: 16px; border-radius: 2px; background: #e94560;
  }}

  /* Bar chart */
  .chart-container {{
    display: flex; align-items: flex-end; gap: 4px;
    height: 120px; padding: 8px 0;
  }}
  .chart-bar-wrap {{
    flex: 1; display: flex; flex-direction: column; align-items: center;
    justify-content: flex-end; height: 100%;
  }}
  .chart-bar {{
    width: 100%; border-radius: 3px 3px 0 0;
    transition: height 0.3s ease;
  }}
  .chart-label {{
    font-size: 9px; color: #484f58; margin-top: 4px;
    writing-mode: horizontal-tb;
  }}

  /* Tables */
  table {{ width: 100%; border-collapse: collapse; font-size: 13px; }}
  th {{
    text-align: left; padding: 8px 12px; font-weight: 600;
    color: #8b949e; border-bottom: 1px solid #21262d;
    font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px;
  }}
  td {{ padding: 8px 12px; border-bottom: 1px solid #21262d; }}
  tr:hover {{ background: #1c2128; }}
  .num {{ text-align: right; font-variant-numeric: tabular-nums; }}
  .mono {{ font-family: 'Cascadia Code', 'Fira Code', monospace; font-size: 12px; }}
  .dim {{ color: #484f58; }}

  .status-dot {{
    display: inline-block; width: 8px; height: 8px; border-radius: 50%;
    margin-right: 6px; vertical-align: middle;
  }}

  /* Event feed */
  .event-feed {{ max-height: 340px; overflow-y: auto; }}
  .event-feed::-webkit-scrollbar {{ width: 6px; }}
  .event-feed::-webkit-scrollbar-track {{ background: #0d1117; }}
  .event-feed::-webkit-scrollbar-thumb {{ background: #30363d; border-radius: 3px; }}
  .event-item {{
    display: flex; align-items: center; gap: 8px;
    padding: 8px 0; border-bottom: 1px solid #21262d;
    font-size: 13px;
  }}
  .event-text {{ flex: 1; color: #c9d1d9; }}
  .event-time {{ font-size: 11px; color: #484f58; font-family: monospace; white-space: nowrap; }}

  /* Grid layout */
  .two-col {{ display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }}
  @media (max-width: 768px) {{ .two-col {{ grid-template-columns: 1fr; }} }}

  /* Footer */
  footer {{
    text-align: center; padding: 16px 0; margin-top: 16px;
    border-top: 1px solid #21262d; color: #484f58; font-size: 11px;
  }}
</style>
</head>
<body>
<div class="container">
  <header>
    <div>
      <h1>{title}</h1>
      <div class="subtitle">Last updated: {now.strftime('%Y-%m-%d %H:%M:%S UTC')}</div>
    </div>
    <div class="refresh-badge">
      <div class="refresh-dot"></div>
      Auto-refresh {refresh_seconds}s
    </div>
  </header>

  <div class="stat-grid">
    <div class="stat-card">
      <div class="accent" style="background: linear-gradient(90deg, #58a6ff, #1f6feb)"></div>
      <div class="num">{metrics['total_agents']}</div>
      <div class="label">Total Agents</div>
      <div class="sub">{sc['online']} online &middot; {sc['offline']} offline</div>
    </div>
    <div class="stat-card">
      <div class="accent" style="background: linear-gradient(90deg, #e94560, #f85149)"></div>
      <div class="num">{metrics['total_tasks']}</div>
      <div class="label">Total Tasks</div>
      <div class="sub">{metrics['tasks_24h']} in 24h &middot; {metrics['tasks_per_hour']}/hr</div>
    </div>
    <div class="stat-card">
      <div class="accent" style="background: linear-gradient(90deg, #ffab40, #f0883e)"></div>
      <div class="num">{metrics['total_creds']}</div>
      <div class="label">Credentials</div>
      <div class="sub">{metrics['creds_per_day']}/day avg</div>
    </div>
    <div class="stat-card">
      <div class="accent" style="background: linear-gradient(90deg, #00e676, #2ea043)"></div>
      <div class="num">{metrics['total_listeners']}</div>
      <div class="label">Listeners</div>
      <div class="sub">{sum(1 for l in metrics['listeners'] if l['status']=='active')} active</div>
    </div>
    <div class="stat-card">
      <div class="accent" style="background: linear-gradient(90deg, #e040fb, #bc8cff)"></div>
      <div class="num">{metrics['hosts_count']}</div>
      <div class="label">Network Hosts</div>
      <div class="sub">discovered</div>
    </div>
  </div>

  <div class="section">
    <div class="section-title">Task Throughput (Last 24h)</div>
    <div class="chart-container">
      {chart_bars}
    </div>
  </div>

  <div class="two-col">
    <div class="section">
      <div class="section-title">Recent Events</div>
      <div class="event-feed">
        {event_items}
      </div>
    </div>
    <div class="section">
      <div class="section-title">Agent Status</div>
      <table>
        <tr><th>Status</th><th style="text-align:right">Count</th><th style="text-align:right">%</th></tr>
        {agent_rows}
      </table>
    </div>
  </div>

  <div class="two-col">
    <div class="section">
      <div class="section-title">Recent Tasks</div>
      <table>
        <tr><th>ID</th><th>Type</th><th>Status</th><th>Time</th></tr>
        {task_rows}
      </table>
    </div>
    <div class="section">
      <div class="section-title">Recent Audit Log</div>
      <table>
        <tr><th>Action</th><th>Resource</th><th>User</th><th>Time</th></tr>
        {audit_rows}
      </table>
    </div>
  </div>

  <div class="two-col">
    <div class="section">
      <div class="section-title">Listeners</div>
      <table>
        <tr><th>ID</th><th>Name</th><th>Type</th><th>Port</th><th>Status</th></tr>
        {listener_rows}
      </table>
    </div>
    <div class="section">
      <div class="section-title">OS Distribution</div>
      <table>
        <tr><th>Operating System</th><th style="text-align:right">Count</th></tr>
        {os_rows}
      </table>
    </div>
  </div>

  <div class="section">
    <div class="section-title">Geographic Distribution</div>
    <table>
      <tr><th>Country</th><th style="text-align:right">Agents</th></tr>
      {geo_rows}
    </table>
  </div>

  <footer>ForgeC2 Real-Time Dashboard &middot; Auto-refreshes every {refresh_seconds}s</footer>
</div>

<script>
  document.addEventListener('DOMContentLoaded', function() {{
    var cards = document.querySelectorAll('.stat-card');
    cards.forEach(function(card, i) {{
      card.style.opacity = '0';
      card.style.transform = 'translateY(12px)';
      setTimeout(function() {{
        card.style.transition = 'opacity 0.35s ease, transform 0.35s ease';
        card.style.opacity = '1';
        card.style.transform = 'translateY(0)';
      }}, 80 * i);
    }});
  }});
</script>
</body>
</html>"""
    return html


def main():
    data_input = read_stdin()
    params = data_input.get("params", {})
    config = data_input.get("config", {})

    fmt = params.get("format", "html")
    title = params.get("title", "Real-Time Dashboard")
    refresh_seconds = int(params.get("refresh_seconds", 30))

    try:
        db = Database()
    except FileNotFoundError as e:
        print(json.dumps({"title": title, "format": fmt, "content": f"Error: {e}"}))
        return

    try:
        agents, tasks, credentials, listeners, audit_logs, network_hosts = collect_data(db)
        metrics = compute_metrics(agents, tasks, credentials, listeners, audit_logs, network_hosts)

        if fmt == "json":
            export = {
                "title": title,
                "generated_at": metrics["now"].isoformat(),
                "agents": {
                    "total": metrics["total_agents"],
                    "status": metrics["status_counts"],
                },
                "tasks": {
                    "total": metrics["total_tasks"],
                    "last_24h": metrics["tasks_24h"],
                    "per_hour": metrics["tasks_per_hour"],
                },
                "credentials": {
                    "total": metrics["total_creds"],
                    "per_day_7d": metrics["creds_per_day"],
                },
                "listeners": metrics["listeners"],
                "geo_distribution": metrics["geo_counts"],
                "os_distribution": metrics["os_counts"],
                "network_hosts": metrics["hosts_count"],
            }
            content = json.dumps(export, indent=2, default=str)
        else:
            content = generate_html(metrics, title, refresh_seconds)
            fmt = "html"

        write_report(title, content, fmt)
    finally:
        db.close()


if __name__ == "__main__":
    main()
