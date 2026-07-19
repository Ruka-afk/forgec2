#!/usr/bin/env python3
"""Infrastructure Health Report plugin — C2 infrastructure health, performance, and reliability metrics."""

import json
import os
import sys
from datetime import datetime, timedelta
from collections import defaultdict

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_report


# ── Health score helpers ────────────────────────────────────────

def _clamp(v, lo=0, hi=100):
    return max(lo, min(hi, int(round(v))))


def _score_agent_health(agents, now):
    if not agents:
        return 100, {"avg_online_minutes": 0, "max_offline_minutes": 0, "stale_count": 0}

    online_count = 0
    offline_count = 0
    stale_count = 0
    online_durations = []
    offline_durations = []

    for a in agents:
        status = a.get("status", "unknown")
        last_seen = a.get("last_seen") or ""
        first_seen = a.get("first_seen") or ""

        last_dt = _parse_dt(last_seen)
        first_dt = _parse_dt(first_seen)

        if status == "online":
            online_count += 1
            if first_dt and last_dt:
                online_durations.append((last_dt - first_dt).total_seconds() / 60)
        else:
            offline_count += 1
            if last_dt:
                off_min = (now - last_dt).total_seconds() / 60
                offline_durations.append(off_min)
                if off_min > 1440:
                    stale_count += 1

    total = len(agents)
    online_pct = (online_count / total * 100) if total else 0
    stale_pct = (stale_count / total * 100) if total else 0
    avg_online = sum(online_durations) / len(online_durations) if online_durations else 0
    max_offline = max(offline_durations) if offline_durations else 0

    score = 100
    score -= stale_pct * 0.8
    if online_pct < 50:
        score -= (50 - online_pct) * 0.5
    if max_offline > 4320:
        score -= 15

    return _clamp(score), {
        "online_count": online_count,
        "offline_count": offline_count,
        "stale_count": stale_count,
        "online_pct": round(online_pct, 1),
        "stale_pct": round(stale_pct, 1),
        "avg_online_minutes": round(avg_online, 1),
        "max_offline_minutes": round(max_offline, 1),
    }


def _score_task_health(tasks, now):
    if not tasks:
        return 100, {"tasks_per_day": 0, "success_rate": 0, "failure_rate": 0, "by_type": {}}

    total = len(tasks)
    completed = sum(1 for t in tasks if t.get("status") == "completed")
    failed = sum(1 for t in tasks if t.get("status") == "failed")
    pending = sum(1 for t in tasks if t.get("status") in ("pending", "queued", "running"))

    success_rate = (completed / total * 100) if total else 0
    failure_rate = (failed / total * 100) if total else 0

    dates = set()
    for t in tasks:
        created = t.get("created_at") or ""
        if created:
            try:
                dt = _parse_dt(created)
                if dt:
                    dates.add(dt.date())
            except Exception:
                pass
    span_days = max(len(dates), 1)
    tasks_per_day = round(total / span_days, 1)

    by_type = defaultdict(lambda: {"total": 0, "completed": 0, "failed": 0})
    for t in tasks:
        tp = t.get("type", "unknown")
        by_type[tp]["total"] += 1
        if t.get("status") == "completed":
            by_type[tp]["completed"] += 1
        elif t.get("status") == "failed":
            by_type[tp]["failed"] += 1

    for v in by_type.values():
        v["success_rate"] = round(v["completed"] / v["total"] * 100, 1) if v["total"] else 0
        v["failure_rate"] = round(v["failed"] / v["total"] * 100, 1) if v["total"] else 0

    score = 100
    if failure_rate > 10:
        score -= (failure_rate - 10) * 1.5
    if tasks_per_day < 1 and total > 0:
        score -= 10

    return _clamp(score), {
        "total": total,
        "completed": completed,
        "failed": failed,
        "pending": pending,
        "success_rate": round(success_rate, 1),
        "failure_rate": round(failure_rate, 1),
        "tasks_per_day": tasks_per_day,
        "active_days": span_days,
        "by_type": dict(by_type),
    }


def _score_listener_health(listeners, agents):
    if not listeners:
        return 100, {"count": 0, "enabled": 0, "agent_distribution": {}}

    enabled = sum(1 for l in listeners if l.get("enabled") or l.get("status") == "active")
    disabled = len(listeners) - enabled

    agent_dist = defaultdict(int)
    for a in agents:
        lid = a.get("listener_id")
        if lid:
            agent_dist[str(lid)] += 1

    max_agents_per = max(agent_dist.values()) if agent_dist else 0
    avg_agents = sum(agent_dist.values()) / len(agent_dist) if agent_dist else 0

    score = 100
    if not enabled:
        score -= 30
    if disabled:
        score -= disabled * 5
    if max_agents_per > 50:
        score -= 10
    if len(listeners) == 1:
        score -= 10

    return _clamp(score), {
        "count": len(listeners),
        "enabled": enabled,
        "disabled": disabled,
        "agent_distribution": dict(agent_dist),
        "max_agents_per_listener": max_agents_per,
        "avg_agents_per_listener": round(avg_agents, 1),
    }


def _score_build_health(builds, now):
    if not builds:
        return 100, {"total": 0, "recent_failures": 0, "failure_rate": 0, "avg_build_seconds": 0}

    total = len(builds)
    recent = [b for b in builds if _parse_dt(b.get("created_at") or "")]
    recent_cutoff = [b for b in recent if (now - (_parse_dt(b.get("created_at") or "") or now)).total_seconds() < 86400 * 7]
    recent_c = recent_cutoff or recent

    failed = sum(1 for b in recent_c if b.get("status") == "failed")
    succeeded = sum(1 for b in recent_c if b.get("status") == "completed" or b.get("status") == "success")
    failure_rate = (failed / len(recent_c) * 100) if recent_c else 0

    durations = []
    for b in recent_c:
        started = _parse_dt(b.get("started_at") or b.get("created_at") or "")
        finished = _parse_dt(b.get("completed_at") or b.get("finished_at") or b.get("updated_at") or "")
        if started and finished:
            durations.append((finished - started).total_seconds())
    avg_seconds = sum(durations) / len(durations) if durations else 0

    score = 100
    if failure_rate > 20:
        score -= (failure_rate - 20) * 1.2
    if failed > 5:
        score -= (failed - 5) * 3

    return _clamp(score), {
        "total": total,
        "recent_total": len(recent_c),
        "recent_failures": failed,
        "recent_succeeded": succeeded,
        "failure_rate": round(failure_rate, 1),
        "avg_build_seconds": round(avg_seconds, 1),
    }


def _score_storage(db):
    tables = {
        "agents": "implants",
        "tasks": "tasks",
        "listeners": "listeners",
        "credentials": "credential_entries",
        "audit_logs": "audit_logs",
        "build_logs": "build_logs",
        "scan_results": "scan_results",
        "network_hosts": "network_hosts",
    }
    row_counts = {}
    total_rows = 0
    for label, table in tables.items():
        try:
            count = db._conn.execute(f"SELECT COUNT(*) FROM {table}").fetchone()[0]
            row_counts[label] = count
            total_rows += count
        except Exception:
            row_counts[label] = 0

    score = 100
    if total_rows > 100000:
        score -= 20
    elif total_rows > 50000:
        score -= 10
    if row_counts.get("audit_logs", 0) > 50000:
        score -= 10

    return _clamp(score), {
        "total_rows": total_rows,
        "by_table": row_counts,
    }


def _compute_overall(scores):
    if not scores:
        return 100
    return _clamp(sum(scores) / len(scores))


def _parse_dt(s):
    if not s:
        return None
    try:
        return datetime.fromisoformat(s.replace("Z", "+00:00")).replace(tzinfo=None)
    except (ValueError, TypeError):
        return None


def _trend_indicator(current, threshold_good=70, threshold_bad=40):
    if current >= threshold_good:
        return "up", "\u2191", "#28a745"
    if current <= threshold_bad:
        return "down", "\u2193", "#dc3545"
    return "stable", "\u2192", "#ffc107"


def _recommendations(agent_m, task_m, listener_m, build_m, storage_m, overall):
    recs = []
    if agent_m.get("stale_count", 0) > 0:
        recs.append(f"{agent_m['stale_count']} agent(s) offline >24h — investigate for detection or network issues.")
    if agent_m.get("online_pct", 100) < 50:
        recs.append("Less than 50% of agents are online — check listener reachability and agent implants.")
    if agent_m.get("max_offline_minutes", 0) > 4320:
        recs.append("Maximum offline duration exceeds 3 days — consider rotating burned agents.")
    if task_m.get("failure_rate", 0) > 20:
        recs.append(f"Task failure rate is {task_m['failure_rate']}% — review failed tasks for config or agent issues.")
    if task_m.get("tasks_per_day", 0) < 1 and task_m.get("total", 0) > 0:
        recs.append("Low task throughput — consider increasing operational tempo.")
    if listener_m.get("disabled", 0) > 0:
        recs.append(f"{listener_m['disabled']} listener(s) disabled — re-enable or remove to reduce confusion.")
    if listener_m.get("count", 0) == 0:
        recs.append("No listeners configured — deploy at least one to accept agent connections.")
    if listener_m.get("count", 0) == 1:
        recs.append("Only one listener active — consider adding a backup for redundancy.")
    if listener_m.get("max_agents_per_listener", 0) > 50:
        recs.append("High agent concentration on a single listener — distribute for load balancing.")
    if build_m.get("failure_rate", 0) > 20:
        recs.append(f"Build failure rate is {build_m['failure_rate']}% — check build pipeline and dependencies.")
    if build_m.get("recent_failures", 0) > 5:
        recs.append("Multiple recent build failures — investigate toolchain issues.")
    if storage_m.get("total_rows", 0) > 100000:
        recs.append("Large dataset detected — consider archiving old records to maintain performance.")
    if overall >= 80:
        recs.append("Infrastructure is healthy — continue regular monitoring.")
    if not recs:
        recs.append("No immediate issues detected — maintain current configuration.")
    return recs


# ── HTML generation ────────────────────────────────────────────

def generate_html(data, title):
    overall = data["overall_score"]
    overall_dir, overall_arrow, overall_color = _trend_indicator(overall)

    def score_card(label, score, detail, metrics_obj):
        d, arrow, color = _trend_indicator(score)
        bars = ""
        for k, v in metrics_obj.items():
            if isinstance(v, (int, float)):
                bars += f'<div class="metric-row"><span class="metric-key">{k.replace("_", " ").title()}</span><span class="metric-val">{v}</span></div>\n'
        return f"""
<div class="score-card">
  <div class="score-header">
    <div class="score-circle" style="border-color:{color}">{score}</div>
    <div><div class="score-label">{label}</div><div class="score-trend" style="color:{color}">{arrow} {d.upper()}</div></div>
  </div>
  <div class="score-body">{bars}</div>
</div>"""

    html = f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>{title}</title>
<style>
body {{ font-family: -apple-system, sans-serif; max-width: 1000px; margin: 0 auto; padding: 20px; color: #1a1a2e; background: #fafafa; }}
h1 {{ color: #0f3460; border-bottom: 3px solid #e94560; padding-bottom: 10px; }}
h2 {{ color: #16213e; margin-top: 30px; border-bottom: 1px solid #ddd; padding-bottom: 5px; }}
.subtitle {{ color: #666; font-size: 14px; margin-bottom: 20px; }}
.overall-box {{ background: #fff; border: 2px solid {overall_color}; border-radius: 12px; padding: 24px; margin: 20px 0; text-align: center; }}
.overall-score {{ font-size: 64px; font-weight: 900; color: {overall_color}; }}
.overall-label {{ font-size: 18px; color: #666; margin-top: 4px; }}
.overall-trend {{ font-size: 16px; font-weight: bold; color: {overall_color}; margin-top: 4px; }}
.grid {{ display: grid; grid-template-columns: repeat(auto-fit, minmax(320px, 1fr)); gap: 16px; margin: 16px 0; }}
.score-card {{ background: #fff; border-radius: 10px; padding: 18px; box-shadow: 0 2px 8px rgba(0,0,0,0.07); }}
.score-header {{ display: flex; align-items: center; gap: 14px; margin-bottom: 12px; }}
.score-circle {{ width: 56px; height: 56px; border-radius: 50%; border: 4px solid; display: flex; align-items: center; justify-content: center; font-size: 22px; font-weight: bold; flex-shrink: 0; }}
.score-label {{ font-size: 15px; font-weight: 600; color: #333; }}
.score-trend {{ font-size: 13px; font-weight: bold; }}
.score-body {{ border-top: 1px solid #f0f0f0; padding-top: 10px; }}
.metric-row {{ display: flex; justify-content: space-between; padding: 3px 0; font-size: 13px; }}
.metric-key {{ color: #666; }}
.metric-val {{ font-weight: 600; color: #333; }}
table {{ width: 100%; border-collapse: collapse; margin: 10px 0; font-size: 12px; }}
th {{ background: #0f3460; color: white; padding: 7px 10px; text-align: left; }}
td {{ border: 1px solid #ddd; padding: 5px 10px; }}
tr:nth-child(even) {{ background: #f8f9fa; }}
.bar {{ background: #e9ecef; border-radius: 4px; height: 14px; overflow: hidden; margin: 3px 0; }}
.bar-fill {{ height: 100%; border-radius: 4px; }}
.bar-label {{ font-size: 11px; color: #666; display: flex; justify-content: space-between; }}
.recs {{ list-style: none; padding: 0; }}
.recs li {{ background: #fff; border-left: 4px solid #0f3460; padding: 10px 14px; margin: 8px 0; border-radius: 4px; box-shadow: 0 1px 2px rgba(0,0,0,0.05); font-size: 14px; }}
.recs li::before {{ content: counter(rec) ". "; font-weight: bold; color: #e94560; }}
.recs {{ counter-reset: rec; }}
.recs li {{ counter-increment: rec; }}
</style></head><body>
<h1>{title}</h1>
<p class="subtitle">Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}</p>

<div class="overall-box">
  <div class="overall-score">{overall}</div>
  <div class="overall-label">Overall Infrastructure Health</div>
  <div class="overall-trend">{overall_arrow} {overall_dir.upper()}</div>
</div>

<h2>Category Health Scores</h2>
<div class="grid">
{score_card("Agent Health", data['agent_score'], "Agent uptime &amp; availability", data['agent'])}
{score_card("Task Throughput", data['task_score'], "Task execution &amp; reliability", {k: v for k, v in data['task'].items() if k != 'by_type'})}
{score_card("Listener Health", data['listener_score'], "Listener availability &amp; load", data['listener'])}
{score_card("Build Health", data['build_score'], "Agent build pipeline", {k: v for k, v in data['build'].items() if k not in ('recent_total',)}) }
{score_card("Storage", data['storage_score'], "Database &amp; log growth", {k: v for k, v in data['storage'].items() if k != 'by_table'})}
</div>"""

    task_by_type = data["task"].get("by_type", {})
    if task_by_type:
        html += "\n\n<h2>Task Failure Rate by Type</h2>\n<table><tr><th>Type</th><th>Total</th><th>Completed</th><th>Failed</th><th>Success %</th></tr>"
        for tp, info in sorted(task_by_type.items(), key=lambda x: -x[1]["failed"]):
            clr = "#28a745" if info["failure_rate"] < 10 else "#dc3545" if info["failure_rate"] > 20 else "#ffc107"
            html += f"\n<tr><td>{tp}</td><td>{info['total']}</td><td>{info['completed']}</td><td>{info['failed']}</td><td style='color:{clr};font-weight:bold'>{info['success_rate']}%</td></tr>"
        html += "\n</table>"

    if data["listener"].get("agent_distribution"):
        html += "\n<h2>Agent Distribution by Listener</h2>\n<table><tr><th>Listener ID</th><th>Agents</th></tr>"
        for lid, count in sorted(data["listener"]["agent_distribution"].items(), key=lambda x: -x[1]):
            html += f"\n<tr><td>{lid}</td><td>{count}</td></tr>"
        html += "\n</table>"

    if data["storage"].get("by_table"):
        html += "\n<h2>Database Row Counts</h2>\n<table><tr><th>Table</th><th>Rows</th></tr>"
        for tbl, cnt in sorted(data["storage"]["by_table"].items(), key=lambda x: -x[1]):
            html += f"\n<tr><td>{tbl}</td><td>{cnt:,}</td></tr>"
        html += f"\n<tr style='font-weight:bold'><td>Total</td><td>{data['storage']['total_rows']:,}</td></tr>\n</table>"

    html += "\n\n<h2>Recommendations</h2>\n<ol class='recs'>"
    for rec in data["recommendations"]:
        html += f"\n  <li>{rec}</li>"
    html += "\n</ol>\n</body></html>"
    return html


# ── Markdown generation ────────────────────────────────────────

def generate_markdown(data, title):
    overall = data["overall_score"]
    _, o_arrow, _ = _trend_indicator(overall)

    lines = [
        f"# {title}",
        f"*Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}*",
        "",
        f"## Overall Health Score: {overall}/100 {o_arrow}",
        "",
        "## Category Scores",
        "| Category | Score | Status |",
        "|----------|-------|--------|",
    ]

    categories = [
        ("Agent Health", data["agent_score"]),
        ("Task Throughput", data["task_score"]),
        ("Listener Health", data["listener_score"]),
        ("Build Health", data["build_score"]),
        ("Storage", data["storage_score"]),
    ]
    for cat, sc in categories:
        _, arrow, _ = _trend_indicator(sc)
        lines.append(f"| {cat} | {sc}/100 | {arrow} |")

    a = data["agent"]
    lines += [
        "",
        "## Agent Health",
        f"- **Online:** {a.get('online_count', 0)} ({a.get('online_pct', 0)}%)",
        f"- **Offline:** {a.get('offline_count', 0)}",
        f"- **Stale (>24h):** {a.get('stale_count', 0)} ({a.get('stale_pct', 0)}%)",
        f"- **Avg Online Duration:** {a.get('avg_online_minutes', 0):.0f} min",
        f"- **Max Offline Duration:** {a.get('max_offline_minutes', 0):.0f} min",
    ]

    t = data["task"]
    lines += [
        "",
        "## Task Throughput",
        f"- **Total Tasks:** {t.get('total', 0)}",
        f"- **Completed:** {t.get('completed', 0)}",
        f"- **Failed:** {t.get('failed', 0)}",
        f"- **Pending:** {t.get('pending', 0)}",
        f"- **Success Rate:** {t.get('success_rate', 0)}%",
        f"- **Failure Rate:** {t.get('failure_rate', 0)}%",
        f"- **Tasks/Day:** {t.get('tasks_per_day', 0)}",
    ]

    by_type = t.get("by_type", {})
    if by_type:
        lines += ["", "### By Type", "| Type | Total | Completed | Failed | Success % |", "|------|-------|-----------|--------|-----------|"]
        for tp, info in sorted(by_type.items(), key=lambda x: -x[1]["failed"]):
            lines.append(f"| {tp} | {info['total']} | {info['completed']} | {info['failed']} | {info['success_rate']}% |")

    l = data["listener"]
    lines += [
        "",
        "## Listener Health",
        f"- **Total Listeners:** {l.get('count', 0)}",
        f"- **Enabled:** {l.get('enabled', 0)}",
        f"- **Disabled:** {l.get('disabled', 0)}",
        f"- **Max Agents/Listener:** {l.get('max_agents_per_listener', 0)}",
    ]

    b = data["build"]
    lines += [
        "",
        "## Build Health",
        f"- **Total Builds:** {b.get('total', 0)}",
        f"- **Recent Builds:** {b.get('recent_total', 0)}",
        f"- **Recent Failures:** {b.get('recent_failures', 0)}",
        f"- **Failure Rate:** {b.get('failure_rate', 0)}%",
        f"- **Avg Build Time:** {b.get('avg_build_seconds', 0):.0f}s",
    ]

    s = data["storage"]
    lines += [
        "",
        "## Storage",
        f"- **Total Rows:** {s.get('total_rows', 0):,}",
    ]
    for tbl, cnt in sorted(s.get("by_table", {}).items(), key=lambda x: -x[1]):
        lines.append(f"  - {tbl}: {cnt:,}")

    lines += ["", "## Recommendations"]
    for i, rec in enumerate(data["recommendations"], 1):
        lines.append(f"{i}. {rec}")

    return "\n".join(lines)


# ── Main ───────────────────────────────────────────────────────

def main():
    data_input = read_stdin()
    params = data_input.get("params", {})
    config = data_input.get("config", {})

    fmt = params.get("format", "html")
    title = params.get("title", "Infrastructure Health Report")

    try:
        db = Database()
    except FileNotFoundError as e:
        print(json.dumps({"title": title, "format": fmt, "content": f"Error: {e}"}))
        return

    now = datetime.utcnow()
    try:
        agents = db.all_agents()
        tasks = db.all_tasks()
        listeners = db.all_listeners()
        builds = db.all_build_logs()
        audit = db.all_audit_logs()

        agent_score, agent_m = _score_agent_health(agents, now)
        task_score, task_m = _score_task_health(tasks, now)
        listener_score, listener_m = _score_listener_health(listeners, agents)
        build_score, build_m = _score_build_health(builds, now)
        storage_score, storage_m = _score_storage(db)

        overall = _compute_overall([agent_score, task_score, listener_score, build_score, storage_score])
        recs = _recommendations(agent_m, task_m, listener_m, build_m, storage_m, overall)

        report_data = {
            "overall_score": overall,
            "agent_score": agent_score,
            "agent": agent_m,
            "task_score": task_score,
            "task": task_m,
            "listener_score": listener_score,
            "listener": listener_m,
            "build_score": build_score,
            "build": build_m,
            "storage_score": storage_score,
            "storage": storage_m,
            "recommendations": recs,
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
