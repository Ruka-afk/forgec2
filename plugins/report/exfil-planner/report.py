#!/usr/bin/env python3
"""Exfiltration Plan Report — staged data, transfer methods, timing, and detection avoidance."""

import json
import os
import re
import sys
from datetime import datetime
from typing import Any, Dict, List, Tuple

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_report


# ── Constants ────────────────────────────────────────────────────────────────

TRANSFER_PROTOCOLS = {
    "https": {"stealth": 5, "bandwidth": "high", "reliability": 5, "label": "HTTPS"},
    "http": {"stealth": 3, "bandwidth": "high", "reliability": 4, "label": "HTTP"},
    "dns": {"stealth": 4, "bandwidth": "low", "reliability": 3, "label": "DNS Tunnel"},
    "smb": {"stealth": 3, "bandwidth": "high", "reliability": 4, "label": "SMB/Share"},
    "icmp": {"stealth": 4, "bandwidth": "low", "reliability": 2, "label": "ICMP"},
    "cloud": {"stealth": 3, "bandwidth": "high", "reliability": 5, "label": "Cloud Upload"},
}

PRIORITY_ORDER = {"credential": 0, "document": 1, "config": 2, "database": 3, "other": 4}

BUSINESS_HOURS = range(9, 18)  # 09:00–18:00 local


# ── Data collection ──────────────────────────────────────────────────────────

def collect_staged_data(tasks: list) -> Tuple[List[dict], float]:
    """Parse download/upload tasks to find files staged for exfil."""
    staged = []
    total_bytes = 0.0

    for t in tasks:
        ttype = (t.get("type") or "").lower()
        status = (t.get("status") or "").lower()
        if status != "completed":
            continue

        params = {}
        raw = t.get("params") or t.get("parameters") or ""
        if isinstance(raw, str):
            try:
                params = json.loads(raw)
            except (json.JSONDecodeError, TypeError):
                params = {}
        elif isinstance(raw, dict):
            params = raw

        output_raw = t.get("output") or t.get("result") or ""
        if isinstance(output_raw, str):
            try:
                output_data = json.loads(output_raw)
            except (json.JSONDecodeError, TypeError):
                output_data = {}
        elif isinstance(raw, dict):
            output_data = output_raw
        else:
            output_data = {}

        filename = (
            params.get("filename")
            or params.get("path")
            or params.get("file")
            or ""
        )
        size = 0
        if isinstance(output_data, dict):
            size = output_data.get("size") or output_data.get("file_size") or 0
        if not size:
            size_match = re.search(r"(\d[\d,]*)\s*(?:bytes|B)", str(output_raw))
            if size_match:
                size = int(size_match.group(1).replace(",", ""))

        if filename and size:
            cat = _categorize_file(filename)
            staged.append({
                "filename": filename,
                "source_agent": (t.get("agent_id") or "")[:8],
                "agent_id": t.get("agent_id"),
                "size_bytes": size,
                "size_human": _human_size(size),
                "category": cat,
                "task_id": t.get("id"),
            })
            total_bytes += size

    staged.sort(key=lambda x: PRIORITY_ORDER.get(x["category"], 4))
    return staged, total_bytes


def _categorize_file(name: str) -> str:
    name_lower = name.lower()
    cred_kw = ("password", "credential", "hash", "token", "key", "secret", "ntlm")
    doc_kw = (".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".csv", ".txt", ".rtf")
    cfg_kw = (".conf", ".cfg", ".ini", ".xml", ".json", ".yaml", ".yml", ".env")
    db_kw = (".db", ".sqlite", ".mdb", ".sql", ".bak", "database")

    if any(k in name_lower for k in cred_kw):
        return "credential"
    if any(name_lower.endswith(k) for k in doc_kw):
        return "document"
    if any(name_lower.endswith(k) for k in cfg_kw):
        return "config"
    if any(k in name_lower for k in db_kw):
        return "database"
    return "other"


def _human_size(n: float) -> str:
    for unit in ("B", "KB", "MB", "GB", "TB"):
        if abs(n) < 1024:
            return f"{n:.1f} {unit}"
        n /= 1024
    return f"{n:.1f} PB"


# ── Transfer method analysis ─────────────────────────────────────────────────

def available_protocols(agent: dict) -> list:
    """Determine which protocols an agent can use for exfil."""
    protocols = ["https", "http"]
    os_lower = (agent.get("os") or "").lower()
    if "windows" in os_lower:
        protocols.append("smb")
    protocols.append("dns")
    protocols.append("icmp")
    protocols.append("cloud")
    return protocols


def estimate_bandwidth(agent: dict) -> dict:
    """Estimate effective bandwidth from agent network type."""
    network = (agent.get("network") or agent.get("network_type") or "").lower()
    interval = agent.get("sleep") or agent.get("interval") or 30
    try:
        interval = int(interval)
    except (ValueError, TypeError):
        interval = 30

    if "wifi" in network or "lan" in network or "ethernet" in network:
        base_mbps = 10
    elif "corporate" in network or "enterprise" in network:
        base_mbps = 5
    else:
        base_mbps = 2

    effective_kbps = (base_mbps * 1024) / max(interval / 10, 1)
    return {
        "base_mbps": base_mbps,
        "interval_sec": interval,
        "effective_kbps": round(effective_kbps, 1),
        "network_type": network or "unknown",
    }


# ── Detection risk ───────────────────────────────────────────────────────────

def assess_detection_risk(agent: dict) -> Tuple[str, str, dict]:
    """Return (level, reason, details) for detection risk of a single agent."""
    network = (agent.get("network") or agent.get("network_type") or "").lower()
    domain = (agent.get("domain") or "").lower()
    os_lower = (agent.get("os") or "").lower()

    risk_score = 0
    factors = []

    monitored_kw = ("corporate", "enterprise", "monitored", "security", "siem")
    if any(k in network for k in monitored_kw):
        risk_score += 3
        factors.append("corporate/monitored network")

    if "dc" in domain or "admin" in domain:
        risk_score += 2
        factors.append("high-value domain")

    if "server" in os_lower or "linux" in os_lower:
        risk_score += 1
        factors.append("server OS")

    if risk_score >= 4:
        return "HIGH", "Agent on heavily monitored infrastructure", {"score": risk_score, "factors": factors}
    if risk_score >= 2:
        return "MEDIUM", "Agent on network with moderate monitoring", {"score": risk_score, "factors": factors}
    return "LOW", "Agent on unmonitored or low-security network", {"score": risk_score, "factors": factors}


# ── Timing analysis ──────────────────────────────────────────────────────────

def recommend_timing(agents: list) -> dict:
    """Recommend exfiltration windows based on agent activity patterns."""
    activity_hours = {}
    for a in agents:
        ls = a.get("last_seen") or ""
        if ls:
            try:
                dt = datetime.fromisoformat(ls.replace("Z", "+00:00"))
                h = dt.hour
                activity_hours[h] = activity_hours.get(h, 0) + 1
            except (ValueError, TypeError):
                pass

    low_activity = sorted(
        [h for h in range(24) if activity_hours.get(h, 0) <= 1],
        key=lambda h: activity_hours.get(h, 0),
    )
    recommended = [h for h in low_activity if h not in BUSINESS_HOURS][:5]

    if not recommended:
        recommended = [2, 3, 4, 5, 6]

    return {
        "recommended_hours": sorted(recommended),
        "avoid_hours": list(BUSINESS_HOURS),
        "activity_map": dict(sorted(activity_hours.items())),
    }


# ── Priority list ────────────────────────────────────────────────────────────

def build_priority_list(staged: list, creds: list) -> List[dict]:
    """Build prioritised exfil target list from staged files + credentials."""
    items = []
    seen = set()

    for s in staged:
        key = s.get("filename")
        if key in seen:
            continue
        seen.add(key)
        items.append({
            "target": s["filename"],
            "category": s["category"],
            "size_bytes": s["size_bytes"],
            "size_human": s["size_human"],
            "source_agent": s["source_agent"],
            "priority": PRIORITY_ORDER.get(s["category"], 4),
        })

    for c in creds:
        username = c.get("username") or ""
        if not username:
            continue
        label = f"Credential: {username}"
        if label in seen:
            continue
        seen.add(label)
        items.append({
            "target": label,
            "category": "credential",
            "size_bytes": 0,
            "size_human": "<1 KB",
            "source_agent": (c.get("agent_id") or "")[:8],
            "priority": 0,
        })

    items.sort(key=lambda x: (x["priority"], -x["size_bytes"]))
    return items


# ── Recommendations ──────────────────────────────────────────────────────────

def build_recommendations(priority_list: list, agents: list, total_bytes: float) -> List[str]:
    recs = []
    online = [a for a in agents if (a.get("status") or "").lower() == "online"]
    if not online:
        recs.append("No online agents available — bring agents online before exfiltration")
    if total_bytes > 1024 * 1024 * 1024:
        recs.append("Large dataset (>1 GB) — use chunked transfer with encryption to avoid detection spikes")
    cred_count = sum(1 for p in priority_list if p["category"] == "credential")
    if cred_count:
        recs.append(f"{cred_count} credential(s) prioritised — exfil first for maximum leverage")
    high_risk = sum(1 for a in agents if assess_detection_risk(a)[0] == "HIGH")
    if high_risk:
        recs.append(f"{high_risk} agent(s) on monitored networks — use DNS or HTTPS blending, avoid bulk transfers")
    recs.append("Encrypt all exfil payloads with AES-256 before transfer")
    recs.append("Use traffic shaping — limit transfers to <500 KB per session to blend with normal traffic")
    recs.append("Schedule transfers during off-hours (see Timing section) to reduce analyst attention")
    recs.append("Rotate exfil methods across agents — do not use the same protocol from multiple hosts simultaneously")
    return recs


# ── Agent exfil plan ─────────────────────────────────────────────────────────

def build_agent_plans(agents: list, priority_list: list, timing: dict) -> List[dict]:
    plans = []
    for a in agents:
        if (a.get("status") or "").lower() != "online":
            continue
        protocols = available_protocols(a)
        bandwidth = estimate_bandwidth(a)
        detection_level, detection_reason, detection_details = assess_detection_risk(a)

        if detection_level == "HIGH":
            recommended = "dns"
            stealth_note = "Use DNS tunneling to blend with normal DNS traffic"
        elif detection_level == "MEDIUM":
            recommended = "https"
            stealth_note = "Use HTTPS to blend with web traffic; chunk transfers"
        else:
            recommended = "https"
            stealth_note = "HTTPS provides highest throughput with acceptable stealth"

        agent_targets = [p for p in priority_list if p.get("source_agent") == (a.get("id") or "")[:8]]
        total_size = sum(p["size_bytes"] for p in agent_targets)
        est_time = _estimate_transfer_time(total_size, bandwidth.get("effective_kbps", 100))

        plans.append({
            "agent_id": (a.get("id") or "")[:8],
            "agent_full_id": a.get("id"),
            "hostname": a.get("hostname") or "unknown",
            "ip": a.get("ip") or "",
            "os": a.get("os") or "",
            "detection_risk": detection_level,
            "detection_reason": detection_reason,
            "detection_factors": detection_details.get("factors", []),
            "available_protocols": protocols,
            "recommended_protocol": recommended,
            "stealth_note": stealth_note,
            "bandwidth": bandwidth,
            "targets": agent_targets,
            "total_size_bytes": total_size,
            "total_size_human": _human_size(total_size),
            "estimated_time": est_time,
            "timing_windows": timing["recommended_hours"],
        })
    return plans


def _estimate_transfer_time(size_bytes: float, effective_kbps: float) -> str:
    if not size_bytes:
        return "N/A"
    if effective_kbps <= 0:
        effective_kbps = 100
    seconds = (size_bytes * 8) / (effective_kbps * 1000)
    if seconds < 60:
        return f"{seconds:.0f}s"
    if seconds < 3600:
        return f"{seconds / 60:.1f} min"
    return f"{seconds / 3600:.1f} hr"


# ── HTML generation ──────────────────────────────────────────────────────────

def generate_html(data: dict, title: str) -> str:
    priority_list = data["priority_list"]
    agent_plans = data["agent_plans"]
    timing = data["timing"]
    recommendations = data["recommendations"]
    total_bytes = data["total_bytes"]
    staged = data["staged"]

    cat_colors = {
        "credential": "#dc3545",
        "document": "#fd7e14",
        "config": "#ffc107",
        "database": "#17a2b8",
        "other": "#6c757d",
    }
    risk_colors = {"HIGH": "#dc3545", "MEDIUM": "#ffc107", "LOW": "#28a745"}

    html = f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>{title}</title>
<style>
body {{ font-family: -apple-system, sans-serif; max-width: 1050px; margin: 0 auto; padding: 20px; color: #1a1a2e; background: #fafafa; }}
h1 {{ color: #0f3460; border-bottom: 3px solid #e94560; padding-bottom: 10px; }}
h2 {{ color: #16213e; margin-top: 30px; border-bottom: 1px solid #ddd; padding-bottom: 5px; }}
.grid {{ display: flex; flex-wrap: wrap; gap: 12px; margin: 15px 0; }}
.stat {{ flex: 1; min-width: 130px; background: #fff; border-left: 4px solid #0f3460; padding: 14px; border-radius: 6px; box-shadow: 0 1px 3px rgba(0,0,0,0.08); }}
.stat .num {{ font-size: 26px; font-weight: bold; color: #e94560; }}
.stat .label {{ font-size: 12px; color: #666; margin-top: 2px; }}
table {{ width: 100%; border-collapse: collapse; margin: 10px 0; font-size: 12px; }}
th {{ background: #0f3460; color: white; padding: 7px 10px; text-align: left; }}
td {{ border: 1px solid #ddd; padding: 5px 10px; }}
tr:nth-child(even) {{ background: #f8f9fa; }}
.badge {{ display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: bold; color: white; }}
.p-credential {{ background: {cat_colors['credential']}; }}
.p-document {{ background: {cat_colors['document']}; }}
.p-config {{ background: {cat_colors['config']}; color: #333; }}
.p-database {{ background: {cat_colors['database']}; }}
.p-other {{ background: {cat_colors['other']}; }}
.risk-high {{ background: {risk_colors['HIGH']}; color: white; }}
.risk-medium {{ background: {risk_colors['MEDIUM']}; color: #333; }}
.risk-low {{ background: {risk_colors['LOW']}; color: white; }}
.agent-card {{ background: #fff; border: 1px solid #e0e0e0; border-radius: 8px; padding: 16px; margin: 14px 0; box-shadow: 0 1px 2px rgba(0,0,0,0.05); }}
.agent-title {{ font-size: 15px; font-weight: bold; color: #0f3460; margin-bottom: 8px; }}
.agent-meta {{ display: flex; gap: 14px; flex-wrap: wrap; font-size: 12px; color: #555; margin-bottom: 10px; }}
.proto-badge {{ display: inline-block; padding: 2px 8px; margin: 2px; border-radius: 4px; font-size: 11px; background: #e8f4f8; border: 1px solid #0f3460; }}
.proto-rec {{ background: #0f3460; color: white; font-weight: bold; }}
.timeline-bar {{ display: flex; gap: 2px; margin: 10px 0; }}
.hour {{ width: 3.5%; height: 30px; border-radius: 3px; text-align: center; font-size: 10px; line-height: 30px; }}
.hour-avoid {{ background: #f8d7da; color: #721c24; }}
.hour-ok {{ background: #d4edda; color: #155724; }}
.hour-rec {{ background: #0f3460; color: white; font-weight: bold; }}
.recs {{ list-style: none; padding: 0; }}
.recs li {{ background: #fff; border-left: 4px solid #0f3460; padding: 10px 15px; margin: 8px 0; border-radius: 4px; box-shadow: 0 1px 2px rgba(0,0,0,0.05); font-size: 13px; }}
.recs li::before {{ content: counter(rec) ". "; font-weight: bold; color: #e94560; }}
.recs {{ counter-reset: rec; }}
.recs li {{ counter-increment: rec; }}
.priority-bar {{ height: 8px; border-radius: 4px; margin-top: 4px; }}
</style></head><body>
<h1>{title}</h1>
<p style="color:#666">Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}</p>

<h2>Summary</h2>
<div class="grid">
  <div class="stat"><div class="num">{len(staged)}</div><div class="label">Staged Files</div></div>
  <div class="stat"><div class="num">{_human_size(total_bytes)}</div><div class="label">Total Data</div></div>
  <div class="stat"><div class="num">{len(agent_plans)}</div><div class="label">Active Agents</div></div>
  <div class="stat"><div class="num">{len(priority_list)}</div><div class="label">Priority Targets</div></div>
  <div class="stat"><div class="num">{len(timing['recommended_hours'])}</div><div class="label">Safe Hours</div></div>
</div>"""

    if priority_list:
        html += """
<h2>Priority Matrix</h2>
<table><tr><th>#</th><th>Target</th><th>Category</th><th>Size</th><th>Source Agent</th><th>Priority</th></tr>"""
        for i, p in enumerate(priority_list[:30], 1):
            cat_cls = f"p-{p['category']}"
            html += f"""<tr>
<td>{i}</td><td>{p['target']}</td>
<td><span class="badge {cat_cls}">{p['category'].upper()}</span></td>
<td>{p['size_human']}</td><td>{p['source_agent']}</td>
<td><div class="priority-bar" style="width:{(5 - p['priority']) * 20}%;background:{cat_colors.get(p['category'], '#6c757d')};"></div></td>
</tr>"""
        html += "\n</table>"

    html += """
<h2>Transfer Timing</h2>
<div class="timeline-bar">"""
    for h in range(24):
        cls = "hour-ok"
        label = str(h)
        if h in BUSINESS_HOURS:
            cls = "hour-avoid"
        elif h in timing["recommended_hours"]:
            cls = "hour-rec"
        html += f'<div class="hour {cls}" title="Hour {h}:00">{label}</div>'
    html += """
</div>
<div style="font-size:11px;color:#666;margin-top:5px;">
  <span style="color:#28a745;">&#9608;</span> Safe &nbsp;
  <span style="color:#dc3545;">&#9608;</span> Avoid (business hours) &nbsp;
  <span style="color:#0f3460;">&#9608;</span> Recommended
</div>"""

    if agent_plans:
        html += "\n\n<h2>Agent Exfiltration Plans</h2>"
        for plan in agent_plans:
            risk_cls = f"risk-{plan['detection_risk'].lower()}"
            html += f"""
<div class="agent-card">
  <div class="agent-title">{plan['hostname']} <span class="badge {risk_cls}">{plan['detection_risk']} RISK</span></div>
  <div class="agent-meta">
    <span>ID: {plan['agent_id']}</span>
    <span>IP: {plan['ip']}</span>
    <span>OS: {plan['os']}</span>
    <span>Data: {plan['total_size_human']}</span>
    <span>Est. Time: {plan['estimated_time']}</span>
  </div>
  <div style="font-size:12px;color:#555;margin-bottom:6px;"><strong>Detection:</strong> {plan['detection_reason']}</div>
  <div style="margin-bottom:6px;"><strong>Protocols:</strong>"""
            for proto in plan["available_protocols"]:
                rec_cls = " proto-rec" if proto == plan["recommended_protocol"] else ""
                label = TRANSFER_PROTOCOLS.get(proto, {}).get("label", proto.upper())
                html += f' <span class="proto-badge{rec_cls}">{label}</span>'
            html += f"""
  </div>
  <div style="font-size:12px;margin-bottom:6px;"><strong>Recommended:</strong> {plan['recommended_protocol'].upper()} — {plan['stealth_note']}</div>
  <div style="font-size:11px;color:#888;">Bandwidth: {plan['bandwidth']['effective_kbps']} KB/s effective ({plan['bandwidth']['network_type']})</div>"""
            if plan["targets"]:
                html += """
  <div style="margin-top:8px;"><strong>Targets:</strong></div>
  <table style="font-size:11px;"><tr><th>File</th><th>Category</th><th>Size</th></tr>"""
                for t in plan["targets"][:10]:
                    html += f"<tr><td>{t['target']}</td><td>{t['category']}</td><td>{t['size_human']}</td></tr>"
                html += "\n  </table>"
            html += "\n</div>"

    html += "\n\n<h2>Recommendations</h2>\n<ol class='recs'>"
    for rec in recommendations:
        html += f"\n  <li>{rec}</li>"
    html += "\n</ol>\n</body></html>"
    return html


# ── Markdown generation ──────────────────────────────────────────────────────

def generate_markdown(data: dict, title: str) -> str:
    priority_list = data["priority_list"]
    agent_plans = data["agent_plans"]
    timing = data["timing"]
    recommendations = data["recommendations"]
    total_bytes = data["total_bytes"]

    lines = [
        f"# {title}",
        f"*Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}*",
        "",
        "## Summary",
        f"- **Staged Files:** {len(data['staged'])}",
        f"- **Total Data:** {_human_size(total_bytes)}",
        f"- **Active Agents:** {len(agent_plans)}",
        f"- **Priority Targets:** {len(priority_list)}",
        f"- **Safe Exfil Hours:** {len(timing['recommended_hours'])}",
        "",
        "## Priority Matrix",
        "| # | Target | Category | Size | Source Agent |",
        "|---|---|---|---|---|",
    ]
    for i, p in enumerate(priority_list[:30], 1):
        lines.append(f"| {i} | {p['target']} | {p['category'].upper()} | {p['size_human']} | {p['source_agent']} |")

    lines += [
        "",
        "## Transfer Timing",
        f"- **Recommended hours:** {', '.join(f'{h}:00' for h in sorted(timing['recommended_hours']))}",
        f"- **Avoid (business hours):** {', '.join(f'{h}:00' for h in BUSINESS_HOURS)}",
        "",
        "## Agent Plans",
    ]
    for plan in agent_plans:
        lines.append(f"\n### {plan['hostname']} [{plan['detection_risk']} RISK]")
        lines.append(f"- **ID:** {plan['agent_id']}")
        lines.append(f"- **IP:** {plan['ip']} | **OS:** {plan['os']}")
        lines.append(f"- **Detection:** {plan['detection_reason']}")
        lines.append(f"- **Recommended:** {plan['recommended_protocol'].upper()} — {plan['stealth_note']}")
        lines.append(f"- **Data:** {plan['total_size_human']} | **Est. Time:** {plan['estimated_time']}")
        lines.append(f"- **Protocols:** {', '.join(plan['available_protocols'])}")
        if plan["targets"]:
            lines.append("\n| File | Category | Size |")
            lines.append("|---|---|---|")
            for t in plan["targets"][:10]:
                lines.append(f"| {t['target']} | {t['category']} | {t['size_human']} |")

    lines += ["", "## Recommendations", ""]
    for i, rec in enumerate(recommendations, 1):
        lines.append(f"{i}. {rec}")

    return "\n".join(lines)


# ── Main ─────────────────────────────────────────────────────────────────────

def main():
    data_input = read_stdin()
    params = data_input.get("params", {})
    fmt = params.get("format", "html")
    title = params.get("title", "Exfiltration Plan")

    try:
        db = Database()
    except FileNotFoundError as e:
        print(json.dumps({"title": title, "format": fmt, "content": f"Error: {e}"}))
        return

    try:
        agents = db.all_agents()
        tasks = db.all_tasks()
        creds = db.all_credentials()
        network_hosts = db.all_network_hosts()

        staged, total_bytes = collect_staged_data(tasks)
        priority_list = build_priority_list(staged, creds)
        timing = recommend_timing(agents)
        recommendations = build_recommendations(priority_list, agents, total_bytes)
        agent_plans = build_agent_plans(agents, priority_list, timing)

        report_data = {
            "staged": staged,
            "total_bytes": total_bytes,
            "priority_list": priority_list,
            "timing": timing,
            "recommendations": recommendations,
            "agent_plans": agent_plans,
            "network_hosts_count": len(network_hosts),
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
