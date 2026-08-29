#!/usr/bin/env python3
"""Executive Summary Report plugin — high-level overview for management and stakeholders."""

import json
import os
import sys
from datetime import datetime, timedelta

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_report


def compute_risk_assessment(metrics: dict) -> tuple:
    """Return (level, reason) based on collected metrics."""
    if metrics.get("burned_agents", 0) > 0:
        return "CRITICAL", "One or more agents offline >24h — potential detection"
    stale_pct = metrics.get("stale_pct", 0)
    if stale_pct > 50:
        return "HIGH", f"{stale_pct:.0f}% of agents are stale (no recent activity)"
    if metrics.get("total_creds", 0) > 0 and metrics.get("unused_creds", 0) > 0:
        return "MEDIUM", "Credentials harvested but not yet leveraged in tasks"
    return "LOW", "Operations proceeding within normal parameters"


def compute_recommendations(metrics: dict) -> list:
    recs = []
    if metrics.get("burned_agents", 0) > 0 or metrics.get("offline_agents", 0) > 0:
        recs.append("Investigate offline agents for potential detection or network issues")
    if metrics.get("total_creds", 0) == 0:
        recs.append("Focus on credential harvesting to expand access")
    if metrics.get("stale_pct", 0) > 50:
        recs.append("Review stale agents — rotate or decommission to reduce noise")
    if metrics.get("geo_unique", 1) <= 1 and metrics.get("total_agents", 0) > 1:
        recs.append("Diversify geographic distribution of agents")
    failed = metrics.get("failed_tasks", 0)
    total_tasks = metrics.get("total_tasks", 0)
    if total_tasks > 0 and failed / total_tasks > 0.2:
        recs.append("Review failed tasks for infrastructure or configuration issues")
    if metrics.get("total_listeners", 0) == 0:
        recs.append("No active listeners — deploy one to accept new agents")
    if not recs:
        recs.append("Continue monitoring operations and maintain current posture")
    return recs


def build_metrics(agents, tasks, creds, listeners, audit, hours):
    now = datetime.utcnow()
    cutoff = None
    if hours > 0:
        cutoff = now - timedelta(hours=hours)

    total_agents = len(agents)
    online = 0
    offline = 0
    stale = 0
    burned = 0
    os_counts = {}
    geo_set = set()

    for a in agents:
        if cutoff:
            ls = a.get("last_seen") or ""
            if ls:
                try:
                    if datetime.fromisoformat(ls.replace("Z", "+00:00")).replace(tzinfo=None) < cutoff:
                        continue
                except (ValueError, TypeError) as exc:
                    print(json.dumps({"level": "error", "message": f"Plugin error: invalid agent last_seen timestamp '{ls}': {exc}"}), file=sys.stderr)

        s = a.get("status", "unknown")
        if s == "online":
            online += 1
        elif s == "offline":
            offline += 1
        elif s == "stale":
            stale += 1

        ls = a.get("last_seen") or ""
        if ls and s == "offline":
            try:
                last = datetime.fromisoformat(ls.replace("Z", "+00:00")).replace(tzinfo=None)
                if (now - last) > timedelta(hours=24):
                    burned += 1
            except (ValueError, TypeError) as exc:
                print(json.dumps({"level": "error", "message": f"Plugin error: cannot compute offline duration from last_seen '{ls}': {exc}"}), file=sys.stderr)

        o = a.get("os") or "unknown"
        os_counts[o] = os_counts.get(o, 0) + 1

        country = a.get("country") or a.get("geo") or ""
        if country:
            geo_set.add(country)

    total_tasks = len(tasks)
    completed = sum(1 for t in tasks if t.get("status") == "completed")
    failed = sum(1 for t in tasks if t.get("status") == "failed")
    success_rate = (completed / total_tasks * 100) if total_tasks else 0

    task_types = {}
    for t in tasks:
        tp = t.get("type", "unknown")
        task_types[tp] = task_types.get(tp, 0) + 1

    total_creds = len(creds)
    cred_types = {}
    for c in creds:
        ct = c.get("type") or "unknown"
        cred_types[ct] = cred_types.get(ct, 0) + 1

    unused_creds = total_creds
    cred_tasks = [t for t in tasks if "credential" in (t.get("type") or "").lower() or "cred" in (t.get("type") or "").lower()]
    if cred_tasks:
        unused_creds = max(0, total_creds - len(cred_tasks))

    first_seen = None
    last_seen = None
    for a in agents:
        fs = a.get("first_seen") or ""
        ls = a.get("last_seen") or ""
        try:
            if fs:
                dt = datetime.fromisoformat(fs.replace("Z", "+00:00")).replace(tzinfo=None)
                if first_seen is None or dt < first_seen:
                    first_seen = dt
        except (ValueError, TypeError) as exc:
            print(json.dumps({"level": "error", "message": f"Plugin error: invalid agent first_seen timestamp '{fs}': {exc}"}), file=sys.stderr)
        try:
            if ls:
                dt = datetime.fromisoformat(ls.replace("Z", "+00:00")).replace(tzinfo=None)
                if last_seen is None or dt > last_seen:
                    last_seen = dt
        except (ValueError, TypeError) as exc:
            print(json.dumps({"level": "error", "message": f"Plugin error: invalid agent last_seen timestamp '{ls}': {exc}"}), file=sys.stderr)

    stale_pct = ((stale + burned) / total_agents * 100) if total_agents else 0

    metrics = {
        "total_agents": total_agents,
        "online": online,
        "offline": offline,
        "stale": stale,
        "burned_agents": burned,
        "stale_pct": stale_pct,
        "os_counts": os_counts,
        "geo_count": len(geo_set),
        "total_tasks": total_tasks,
        "completed_tasks": completed,
        "failed_tasks": failed,
        "success_rate": success_rate,
        "task_types": task_types,
        "total_creds": total_creds,
        "unused_creds": unused_creds,
        "cred_types": cred_types,
        "total_listeners": len(listeners),
        "first_seen": first_seen.isoformat() if first_seen else "N/A",
        "last_seen": last_seen.isoformat() if last_seen else "N/A",
    }
    return metrics


def generate_html(metrics: dict, risk_level: str, risk_reason: str, recommendations: list, title: str) -> str:
    risk_colors = {
        "CRITICAL": ("#dc3545", "#f8d7da"),
        "HIGH": ("#fd7e14", "#fff3cd"),
        "MEDIUM": ("#ffc107", "#fff3cd"),
        "LOW": ("#28a745", "#d4edda"),
    }
    color, bg = risk_colors.get(risk_level, ("#6c757d", "#e2e3e5"))

    html = f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>{title}</title>
<style>
body {{ font-family: -apple-system, sans-serif; max-width: 900px; margin: 0 auto; padding: 20px; color: #1a1a2e; background: #fafafa; }}
h1 {{ color: #0f3460; border-bottom: 3px solid #e94560; padding-bottom: 10px; }}
h2 {{ color: #16213e; margin-top: 30px; border-bottom: 1px solid #ddd; padding-bottom: 5px; }}
.subtitle {{ color: #666; font-size: 14px; }}
.grid {{ display: flex; flex-wrap: wrap; gap: 12px; margin: 15px 0; }}
.stat {{ flex: 1; min-width: 140px; background: #fff; border-left: 4px solid #0f3460; padding: 15px; border-radius: 6px; box-shadow: 0 1px 3px rgba(0,0,0,0.08); }}
.stat .num {{ font-size: 28px; font-weight: bold; color: #e94560; }}
.stat .label {{ font-size: 12px; color: #666; margin-top: 2px; }}
.risk-badge {{ display: inline-block; padding: 6px 18px; border-radius: 20px; font-weight: bold; font-size: 14px; color: white; background: {color}; }}
.risk-box {{ background: {bg}; border: 1px solid {color}; border-radius: 8px; padding: 15px 20px; margin: 15px 0; }}
.risk-reason {{ color: #333; margin-top: 5px; font-size: 14px; }}
.recs {{ list-style: none; padding: 0; }}
.recs li {{ background: #fff; border-left: 4px solid #0f3460; padding: 10px 15px; margin: 8px 0; border-radius: 4px; box-shadow: 0 1px 2px rgba(0,0,0,0.05); font-size: 14px; }}
.recs li::before {{ content: counter(rec) ". "; font-weight: bold; color: #e94560; }}
.recs {{ counter-reset: rec; }}
.recs li {{ counter-increment: rec; }}
.bar {{ background: #e9ecef; border-radius: 4px; height: 16px; overflow: hidden; margin: 4px 0; }}
.bar-fill {{ height: 100%; border-radius: 4px; background: #0f3460; }}
.bar-label {{ font-size: 11px; color: #666; display: flex; justify-content: space-between; }}
table {{ width: 100%; border-collapse: collapse; margin: 10px 0; }}
th, td {{ border: 1px solid #ddd; padding: 6px 10px; text-align: left; font-size: 12px; }}
th {{ background: #0f3460; color: white; }}
tr:nth-child(even) {{ background: #f8f9fa; }}
</style></head><body>
<h1>{title}</h1>
<p class="subtitle">Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')} | Time Window: {'All time' if not any(True for _ in []) else 'Filtered'}</p>

<h2>Risk Assessment</h2>
<div class="risk-box">
  <span class="risk-badge">{risk_level}</span>
  <p class="risk-reason">{risk_reason}</p>
</div>

<h2>Key Metrics</h2>
<div class="grid">
  <div class="stat"><div class="num">{metrics['total_agents']}</div><div class="label">Total Agents</div></div>
  <div class="stat"><div class="num">{metrics['online']}</div><div class="label">Online</div></div>
  <div class="stat"><div class="num">{metrics['offline']}</div><div class="label">Offline</div></div>
  <div class="stat"><div class="num">{metrics['stale']}</div><div class="label">Stale</div></div>
  <div class="stat"><div class="num">{metrics['total_tasks']}</div><div class="label">Tasks Executed</div></div>
  <div class="stat"><div class="num">{metrics['success_rate']:.1f}%</div><div class="label">Success Rate</div></div>
  <div class="stat"><div class="num">{metrics['total_creds']}</div><div class="label">Credentials</div></div>
  <div class="stat"><div class="num">{metrics['total_listeners']}</div><div class="label">Listeners</div></div>
</div>

<h2>Agent Status Distribution</h2>"""

    total_a = metrics['total_agents'] or 1
    for label, count, clr in [("Online", metrics['online'], "#28a745"), ("Offline", metrics['offline'], "#dc3545"), ("Stale", metrics['stale'], "#ffc107")]:
        pct = count / total_a * 100
        html += f"""
<div class="bar-label"><span>{label}</span><span>{count} ({pct:.0f}%)</span></div>
<div class="bar"><div class="bar-fill" style="width:{pct}%;background:{clr};"></div></div>"""

    if metrics['os_counts']:
        html += "\n\n<h2>Operating Systems</h2>\n<table><tr><th>OS</th><th>Count</th></tr>"
        for os_name, cnt in sorted(metrics['os_counts'].items(), key=lambda x: -x[1]):
            html += f"<tr><td>{os_name}</td><td>{cnt}</td></tr>"
        html += "</table>"

    if metrics['cred_types']:
        html += "\n<h2>Credentials by Type</h2>\n<table><tr><th>Type</th><th>Count</th></tr>"
        for ct, cnt in sorted(metrics['cred_types'].items(), key=lambda x: -x[1]):
            html += f"<tr><td>{ct}</td><td>{cnt}</td></tr>"
        html += "</table>"

    if metrics['task_types']:
        html += "\n<h2>Task Types</h2>\n<table><tr><th>Type</th><th>Count</th></tr>"
        for tp, cnt in sorted(metrics['task_types'].items(), key=lambda x: -x[1]):
            html += f"<tr><td>{tp}</td><td>{cnt}</td></tr>"
        html += "</table>"

    html += f"""

<h2>Activity Timeline</h2>
<table><tr><th>Metric</th><th>Value</th></tr>
<tr><td>First Agent Activity</td><td>{metrics['first_seen']}</td></tr>
<tr><td>Last Agent Activity</td><td>{metrics['last_seen']}</td></tr>
<tr><td>Geographic Spread</td><td>{metrics['geo_count']} location(s)</td></tr>
</table>

<h2>Recommendations</h2>
<ol class="recs">"""
    for rec in recommendations:
        html += f"\n  <li>{rec}</li>"
    html += "\n</ol>\n</body></html>"
    return html


def generate_markdown(metrics: dict, risk_level: str, risk_reason: str, recommendations: list, title: str) -> str:
    lines = [
        f"# {title}",
        f"*Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}*",
        "",
        "## Risk Assessment",
        f"**Level:** {risk_level}",
        f"> {risk_reason}",
        "",
        "## Key Metrics",
        f"- **Total Agents:** {metrics['total_agents']}",
        f"- **Online:** {metrics['online']}",
        f"- **Offline:** {metrics['offline']}",
        f"- **Stale:** {metrics['stale']}",
        f"- **Tasks Executed:** {metrics['total_tasks']}",
        f"- **Success Rate:** {metrics['success_rate']:.1f}%",
        f"- **Credentials:** {metrics['total_creds']}",
        f"- **Listeners:** {metrics['total_listeners']}",
        "",
        "## Agent Status",
    ]
    total_a = metrics['total_agents'] or 1
    for label, count in [("Online", metrics['online']), ("Offline", metrics['offline']), ("Stale", metrics['stale'])]:
        lines.append(f"- **{label}:** {count} ({count / total_a * 100:.0f}%)")

    if metrics['os_counts']:
        lines += ["", "## Operating Systems", "| OS | Count |", "|---|---|"]
        for os_name, cnt in sorted(metrics['os_counts'].items(), key=lambda x: -x[1]):
            lines.append(f"| {os_name} | {cnt} |")

    if metrics['cred_types']:
        lines += ["", "## Credentials by Type", "| Type | Count |", "|---|---|"]
        for ct, cnt in sorted(metrics['cred_types'].items(), key=lambda x: -x[1]):
            lines.append(f"| {ct} | {cnt} |")

    lines += [
        "", "## Activity Timeline",
        f"- **First Activity:** {metrics['first_seen']}",
        f"- **Last Activity:** {metrics['last_seen']}",
        f"- **Geographic Spread:** {metrics['geo_count']} location(s)",
        "", "## Recommendations",
    ]
    for i, rec in enumerate(recommendations, 1):
        lines.append(f"{i}. {rec}")

    return "\n".join(lines)


def main():
    data_input = read_stdin()
    params = data_input.get("params", {})
    config = data_input.get("config", {})

    fmt = params.get("format", "html")
    title = params.get("title", "Executive Summary")
    hours = int(params.get("hours", 0))

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

        metrics = build_metrics(agents, tasks, creds, listeners, audit, hours)
        risk_level, risk_reason = compute_risk_assessment(metrics)
        recommendations = compute_recommendations(metrics)

        metrics["risk_level"] = risk_level
        metrics["risk_reason"] = risk_reason
        metrics["recommendations"] = recommendations

        if fmt == "json":
            content = json.dumps(metrics, indent=2, default=str)
        elif fmt == "markdown":
            content = generate_markdown(metrics, risk_level, risk_reason, recommendations, title)
        else:
            content = generate_html(metrics, risk_level, risk_reason, recommendations, title)
            fmt = "html"

        write_report(title, content, fmt)
    finally:
        db.close()


if __name__ == "__main__":
    main()
