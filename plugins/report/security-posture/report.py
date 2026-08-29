#!/usr/bin/env python3
"""Security Posture Assessment Report plugin — scores agent, credential, network, task, and infrastructure hygiene."""

import json
import os
import sys
from datetime import datetime, timedelta

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_report


SENSITIVE_TASK_TYPES = {
    "shell", "powershell", "execute", "mimikatz", "hashdump",
    "kerberos", "dcsync", "screenshot", "keylog", "persist",
    "lateral", "psexec", "wmi_exec", "sharphound", "bloodhound",
}


def score_agent_security(agents: list) -> tuple:
    """Score 0-100. Deductions for low integrity, stale agents, legacy OS, running as admin."""
    if not agents:
        return 100, {"total": 0}

    score = 100
    details = {"total": len(agents)}
    now = datetime.utcnow()

    low_integrity = 0
    stale_count = 0
    legacy_os = 0
    elevated = 0
    online = 0

    legacy_keywords = ["xp", "vista", "2003", "2008", "windows 7"]
    stale_cutoff = now - timedelta(hours=48)

    for a in agents:
        status = (a.get("status") or "").lower()
        if status == "online":
            online += 1

        integrity = (a.get("integrity") or "").lower()
        if integrity in ("low", "untrusted", "unknown"):
            low_integrity += 1

        ls = a.get("last_seen") or ""
        if ls:
            try:
                dt = datetime.fromisoformat(ls.replace("Z", "+00:00")).replace(tzinfo=None)
                if dt < stale_cutoff and status != "online":
                    stale_count += 1
            except (ValueError, TypeError) as exc:
                print(json.dumps({"level": "error", "message": f"Plugin error: invalid agent last_seen timestamp '{ls}': {exc}"}), file=sys.stderr)

        os_name = (a.get("os") or "").lower()
        for kw in legacy_keywords:
            if kw in os_name:
                legacy_os += 1
                break

        username = (a.get("username") or "").lower()
        if "admin" in username or "system" in username:
            elevated += 1

    n = len(agents)
    if n > 0:
        if low_integrity > 0:
            score -= min(25, int(low_integrity / n * 40))
        if stale_count > 0:
            score -= min(20, int(stale_count / n * 35))
        if legacy_os > 0:
            score -= min(15, int(legacy_os / n * 25))
        if elevated > 0:
            score -= min(10, int(elevated / n * 15))

    details.update({
        "online": online,
        "low_integrity": low_integrity,
        "stale": stale_count,
        "legacy_os": legacy_os,
        "elevated": elevated,
    })
    return max(0, score), details


def score_credential_hygiene(creds: list) -> tuple:
    """Score 0-100. Deductions for cleartext creds, password reuse, high-value accounts."""
    if not creds:
        return 100, {"total": 0}

    score = 100
    details = {"total": len(creds)}

    cleartext = 0
    by_domain_user = {}
    plaintext_types = {"password", "plaintext", "cleartext", "raw", ""}

    for c in creds:
        ctype = (c.get("type") or "").lower()
        if ctype in plaintext_types:
            cleartext += 1

        domain = (c.get("domain") or "").lower()
        user = (c.get("username") or "").lower()
        key = f"{domain}\\{user}" if domain else user
        by_domain_user.setdefault(key, []).append(c)

    reuse_count = 0
    high_value_no_mfa = 0
    high_value_names = {"admin", "administrator", "krbtgt", "service", "sql", "enterprise"}

    for account, entries in by_domain_user.items():
        if len(entries) > 1:
            reuse_count += len(entries) - 1
        user_part = account.split("\\")[-1]
        for hv in high_value_names:
            if hv in user_part:
                has_mfa = any("mfa" in (e.get("type") or "").lower() for e in entries)
                if not has_mfa:
                    high_value_no_mfa += 1
                break

    n = len(creds)
    if cleartext > 0:
        score -= min(35, int(cleartext / n * 50))
    if reuse_count > 0:
        score -= min(25, int(reuse_count / n * 40))
    if high_value_no_mfa > 0:
        score -= min(20, high_value_no_mfa * 5)

    details.update({
        "cleartext": cleartext,
        "reused": reuse_count,
        "high_value_no_mfa": high_value_no_mfa,
    })
    return max(0, score), details


def score_network_exposure(agents: list, hosts: list) -> tuple:
    """Score 0-100. Deductions for public IPs, exposed hosts."""
    score = 100
    details = {"total_hosts": len(hosts), "total_agents": len(agents)}

    public_agents = sum(1 for a in agents if a.get("public_ip") and a.get("public_ip") not in ("", "0.0.0.0", "N/A"))
    details["public_ip_agents"] = public_agents

    exposed_hosts = 0
    for h in hosts:
        ports = h.get("open_ports") or ""
        if isinstance(ports, str):
            port_list = [p.strip() for p in ports.split(",") if p.strip()]
        elif isinstance(ports, list):
            port_list = ports
        else:
            port_list = []
        if port_list:
            exposed_hosts += 1

    details["exposed_hosts"] = exposed_hosts

    total = len(agents) or 1
    if public_agents > 0:
        score -= min(30, int(public_agents / total * 45))
    if exposed_hosts > 0:
        score -= min(20, min(20, exposed_hosts * 3))

    return max(0, score), details


def score_task_hygiene(tasks: list) -> tuple:
    """Score 0-100. Deductions for high failure rate, sensitive commands."""
    if not tasks:
        return 100, {"total": 0}

    score = 100
    details = {"total": len(tasks)}

    total = len(tasks)
    failed = sum(1 for t in tasks if (t.get("status") or "").lower() == "failed")
    success = sum(1 for t in tasks if (t.get("status") or "").lower() == "completed")

    details["failed"] = failed
    details["completed"] = success
    details["success_rate"] = round(success / total * 100, 1) if total else 0

    if total > 0:
        fail_ratio = failed / total
        if fail_ratio > 0.5:
            score -= 30
        elif fail_ratio > 0.3:
            score -= 20
        elif fail_ratio > 0.1:
            score -= 10

    sensitive_count = 0
    for t in tasks:
        ttype = (t.get("type") or "").lower()
        if ttype in SENSITIVE_TASK_TYPES:
            sensitive_count += 1

    details["sensitive_tasks"] = sensitive_count
    if total > 0 and sensitive_count / total > 0.4:
        score -= 15

    return max(0, score), details


def score_infrastructure(listeners: list) -> tuple:
    """Score 0-100. Deductions for no listeners, plain HTTP, missing SSL."""
    if not listeners:
        return 50, {"total": 0, "reason": "No listeners configured"}

    score = 100
    details = {"total": len(listeners)}

    no_ssl = 0
    plain_http = 0

    for l in listeners:
        protocol = (l.get("protocol") or "").lower()
        ssl_enabled = l.get("ssl_enabled")
        cert = l.get("cert_path") or ""

        if protocol == "http" and not ssl_enabled and not cert:
            plain_http += 1
        if ssl_enabled is False or (ssl_enabled is None and not cert):
            no_ssl += 1

    details["plain_http"] = plain_http
    details["no_ssl"] = no_ssl

    if plain_http > 0:
        score -= min(30, plain_http * 10)
    if no_ssl > 0:
        score -= min(20, no_ssl * 7)

    return max(0, score), details


def radar_chart_text(categories: dict) -> str:
    """Generate a text-based radar/spider chart using ASCII art."""
    labels = list(categories.keys())
    scores = list(categories.values())
    n = len(labels)
    if n == 0:
        return ""

    width = 60
    height = 15
    mid_x = width // 2
    mid_y = height // 2
    radius = min(mid_x, mid_y) - 2

    grid = [[" " for _ in range(width)] for _ in range(height)]

    import math
    angles = []
    for i in range(n):
        angle = math.radians(270 + i * (360 / n))
        angles.append(angle)
        ex = int(mid_x + radius * math.cos(angle))
        ey = int(mid_y + radius * math.sin(angle))
        if 0 <= ex < width and 0 <= ey < height:
            grid[ey][ex] = "+"

    for r in range(1, 5):
        for i in range(n):
            frac = r / 4
            ax = int(mid_x + radius * frac * math.cos(angles[i]))
            ay = int(mid_y + radius * frac * math.sin(angles[i]))
            bx = int(mid_x + radius * frac * math.cos(angles[(i + 1) % n]))
            by = int(mid_y + radius * frac * math.sin(angles[(i + 1) % n]))
            steps = max(abs(bx - ax), abs(by - ay), 1)
            for s in range(steps + 1):
                t = s / steps
                cx = int(ax + t * (bx - ax))
                cy = int(ay + t * (by - ay))
                if 0 <= cx < width and 0 <= cy < height and grid[cy][cx] == " ":
                    grid[cy][cx] = "."

    for i in range(n):
        frac = scores[i] / 100.0
        px = int(mid_x + radius * frac * math.cos(angles[i]))
        py = int(mid_y + radius * frac * math.sin(angles[i]))
        if 0 <= px < width and 0 <= py < height:
            grid[py][px] = "#"

    lines = []
    lines.append("+" + "-" * (width - 2) + "+")
    for row in grid:
        lines.append("|" + "".join(row) + "|")
    lines.append("+" + "-" * (width - 2) + "+")

    legend = "  ".join(f"{k}: {v}" for k, v in categories.items())
    lines.append("")
    lines.append("Legend:  # = data point  . = grid  + = axis")
    lines.append(f"Scores: {legend}")

    return "\n".join(lines)


def overall_score(scores: dict) -> int:
    weights = {
        "Agent Security": 0.25,
        "Credential Hygiene": 0.25,
        "Network Exposure": 0.20,
        "Task Hygiene": 0.15,
        "Infrastructure": 0.15,
    }
    total_w = 0
    weighted = 0
    for k, v in scores.items():
        w = weights.get(k, 0.2)
        weighted += v * w
        total_w += w
    return round(weighted / total_w) if total_w else 0


def grade(score: int) -> str:
    if score >= 90:
        return "A"
    if score >= 80:
        return "B"
    if score >= 70:
        return "C"
    if score >= 60:
        return "D"
    return "F"


def generate_recommendations(categories: dict, details: dict) -> list:
    recs = []

    agent_d = details.get("agent", {})
    if agent_d.get("stale", 0) > 0:
        recs.append(f"Remove or rotate {agent_d['stale']} stale agent(s) — no activity in 48+ hours.")
    if agent_d.get("legacy_os", 0) > 0:
        recs.append(f"{agent_d['legacy_os']} agent(s) run legacy OS — upgrade or isolate.")
    if agent_d.get("low_integrity", 0) > 0:
        recs.append(f"{agent_d['low_integrity']} agent(s) at low integrity — investigate compromise scope.")

    cred_d = details.get("credential", {})
    if cred_d.get("cleartext", 0) > 0:
        recs.append(f"{cred_d['cleartext']} credential(s) stored in cleartext — rotate and encrypt.")
    if cred_d.get("reused", 0) > 0:
        recs.append(f"{cred_d['reused']} credential reuse instance(s) detected — enforce unique passwords.")
    if cred_d.get("high_value_no_mfa", 0) > 0:
        recs.append(f"{cred_d['high_value_no_mfa']} high-value account(s) lack MFA.")

    net_d = details.get("network", {})
    if net_d.get("public_ip_agents", 0) > 0:
        recs.append(f"{net_d['public_ip_agents']} agent(s) expose public IPs — review network segmentation.")
    if net_d.get("exposed_hosts", 0) > 0:
        recs.append(f"{net_d['exposed_hosts']} network host(s) have open ports — audit and close unnecessary services.")

    task_d = details.get("task", {})
    if task_d.get("failed", 0) > 0 and task_d.get("total", 0) > 0:
        fail_pct = task_d["failed"] / task_d["total"] * 100
        if fail_pct > 20:
            recs.append(f"Task failure rate is {fail_pct:.0f}% — review agent stability and command compatibility.")

    infra_d = details.get("infrastructure", {})
    if infra_d.get("plain_http", 0) > 0:
        recs.append(f"{infra_d['plain_http']} listener(s) use plain HTTP — enable TLS encryption.")
    if infra_d.get("total", 0) == 0:
        recs.append("No listeners configured — deploy at least one listener.")

    if not recs:
        recs.append("Security posture is strong. Continue monitoring and maintaining current controls.")

    return recs


def score_color(score: int) -> str:
    if score >= 80:
        return "#28a745"
    if score >= 60:
        return "#ffc107"
    return "#dc3545"


def score_bg(score: int) -> str:
    if score >= 80:
        return "#d4edda"
    if score >= 60:
        return "#fff3cd"
    return "#f8d7da"


def generate_html(title: str, scores: dict, overall: int, details: dict, recommendations: list) -> str:
    g = grade(overall)
    ocolor = score_color(overall)
    obg = score_bg(overall)

    cards_html = ""
    for label, sc in scores.items():
        c = score_color(sc)
        bg = score_bg(sc)
        d = details.get(label.lower().replace(" ", "_"), {})
        items = "".join(f"<li>{k}: {v}</li>" for k, v in d.items() if k != "total" and not k.startswith("_"))
        cards_html += f"""
    <div class="score-card" style="border-left-color:{c};">
      <div class="card-header">
        <span class="card-title">{label}</span>
        <span class="card-score" style="color:{c};">{sc}/100</span>
      </div>
      <div class="card-bar"><div class="card-bar-fill" style="width:{sc}%;background:{c};"></div></div>
      <ul class="card-details">{items}</ul>
    </div>"""

    radar = radar_chart_text(scores).replace("\n", "<br>").replace(" ", "&nbsp;")
    recs_html = "".join(f"<li>{r}</li>" for r in recommendations)

    return f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>{title}</title>
<style>
body {{ font-family: -apple-system, sans-serif; max-width: 960px; margin: 0 auto; padding: 20px; color: #1a1a2e; background: #fafafa; }}
h1 {{ color: #0f3460; border-bottom: 3px solid #e94560; padding-bottom: 10px; }}
h2 {{ color: #16213e; margin-top: 30px; border-bottom: 1px solid #ddd; padding-bottom: 5px; }}
.subtitle {{ color: #666; font-size: 14px; }}
.overall {{ display: flex; align-items: center; gap: 20px; background: {obg}; border: 1px solid {ocolor}; border-radius: 10px; padding: 20px 30px; margin: 15px 0; }}
.overall .big {{ font-size: 56px; font-weight: bold; color: {ocolor}; }}
.overall .grade {{ font-size: 36px; font-weight: bold; color: {ocolor}; }}
.overall .meta {{ font-size: 14px; color: #444; }}
.score-cards {{ display: grid; grid-template-columns: 1fr 1fr; gap: 14px; margin: 15px 0; }}
.score-card {{ background: #fff; border-left: 5px solid; border-radius: 6px; padding: 14px 16px; box-shadow: 0 1px 3px rgba(0,0,0,0.08); }}
.card-header {{ display: flex; justify-content: space-between; align-items: center; }}
.card-title {{ font-weight: 600; font-size: 14px; color: #16213e; }}
.card-score {{ font-size: 20px; font-weight: bold; }}
.card-bar {{ background: #e9ecef; border-radius: 4px; height: 8px; margin: 6px 0; overflow: hidden; }}
.card-bar-fill {{ height: 100%; border-radius: 4px; transition: width 0.3s; }}
.card-details {{ list-style: none; padding: 0; margin: 6px 0 0 0; font-size: 12px; color: #555; }}
.card-details li {{ margin: 2px 0; }}
.radar-box {{ background: #fff; border: 1px solid #ddd; border-radius: 8px; padding: 15px; font-family: monospace; font-size: 11px; line-height: 1.3; overflow-x: auto; white-space: pre; margin: 10px 0; }}
.recs {{ list-style: none; padding: 0; }}
.recs li {{ background: #fff; border-left: 4px solid #0f3460; padding: 10px 15px; margin: 8px 0; border-radius: 4px; box-shadow: 0 1px 2px rgba(0,0,0,0.05); font-size: 14px; }}
.recs li::before {{ content: counter(rec) ". "; font-weight: bold; color: #e94560; }}
.recs {{ counter-reset: rec; }}
.recs li {{ counter-increment: rec; }}
</style></head><body>
<h1>{title}</h1>
<p class="subtitle">Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}</p>

<div class="overall">
  <div class="big">{overall}</div>
  <div class="grade">Grade {g}</div>
  <div class="meta">Overall Security Posture Score</div>
</div>

<h2>Category Scores</h2>
<div class="score-cards">{cards_html}
</div>

<h2>Score Radar</h2>
<div class="radar-box">{radar}</div>

<h2>Recommendations</h2>
<ol class="recs">{recs_html}</ol>

</body></html>"""


def generate_markdown(title: str, scores: dict, overall: int, details: dict, recommendations: list) -> str:
    g = grade(overall)
    lines = [
        f"# {title}",
        f"*Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}*",
        "",
        f"## Overall Score: {overall}/100 (Grade {g})",
        "",
        "## Category Scores",
        "| Category | Score | Status |",
        "|----------|-------|--------|",
    ]
    for label, sc in scores.items():
        status = "Good" if sc >= 80 else "Fair" if sc >= 60 else "Poor"
        lines.append(f"| {label} | {sc}/100 | {status} |")

    lines += ["", "## Details"]
    for label, sc in scores.items():
        key = label.lower().replace(" ", "_")
        d = details.get(key, {})
        lines.append(f"\n### {label} ({sc}/100)")
        for k, v in d.items():
            if k != "total" and not k.startswith("_"):
                lines.append(f"- {k}: {v}")

    lines += ["", "## Recommendations"]
    for i, rec in enumerate(recommendations, 1):
        lines.append(f"{i}. {rec}")

    return "\n".join(lines)


def generate_json(title: str, scores: dict, overall: int, details: dict, recommendations: list) -> str:
    data = {
        "title": title,
        "overall_score": overall,
        "grade": grade(overall),
        "categories": {},
        "recommendations": recommendations,
    }
    for label, sc in scores.items():
        key = label.lower().replace(" ", "_")
        data["categories"][label] = {"score": sc, "details": details.get(key, {})}
    return json.dumps(data, indent=2, default=str)


def main():
    data_input = read_stdin()
    params = data_input.get("params", {})

    fmt = params.get("format", "html")
    title = params.get("title", "Security Posture Assessment")

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
        hosts = db.all_network_hosts()

        agent_score, agent_details = score_agent_security(agents)
        cred_score, cred_details = score_credential_hygiene(creds)
        net_score, net_details = score_network_exposure(agents, hosts)
        task_score, task_details = score_task_hygiene(tasks)
        infra_score, infra_details = score_infrastructure(listeners)

        scores = {
            "Agent Security": agent_score,
            "Credential Hygiene": cred_score,
            "Network Exposure": net_score,
            "Task Hygiene": task_score,
            "Infrastructure": infra_score,
        }
        overall = overall_score(scores)

        details = {
            "agent_security": agent_details,
            "credential_hygiene": cred_details,
            "network_exposure": net_details,
            "task_hygiene": task_details,
            "infrastructure": infra_details,
        }

        recommendations = generate_recommendations(scores, details)

        if fmt == "json":
            content = generate_json(title, scores, overall, details, recommendations)
        elif fmt == "markdown":
            content = generate_markdown(title, scores, overall, details, recommendations)
        else:
            content = generate_html(title, scores, overall, details, recommendations)
            fmt = "html"

        write_report(title, content, fmt)
    finally:
        db.close()


if __name__ == "__main__":
    main()
