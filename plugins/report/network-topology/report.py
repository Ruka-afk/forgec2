#!/usr/bin/env python3
"""Network Topology Report plugin — visualizes agent relationships, subnets, and connectivity."""

import json
import os
import sys
from collections import defaultdict
from datetime import datetime

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_report


def _subnet(ip):
    parts = (ip or "").split(".")
    if len(parts) >= 3:
        return ".".join(parts[:3])
    return "unknown"


def build_topology(agents, listeners, scan_results, network_hosts):
    subnets = defaultdict(list)
    agent_map = {}
    for a in agents:
        ip = a.get("ip") or a.get("public_ip") or "0.0.0.0"
        sn = _subnet(ip)
        entry = {
            "id": a.get("id"),
            "short_id": (a.get("id") or "")[:8],
            "hostname": a.get("hostname") or "unknown",
            "username": a.get("username") or "",
            "ip": ip,
            "public_ip": a.get("public_ip") or "",
            "os": a.get("os") or "unknown",
            "status": a.get("status") or "unknown",
            "integrity": a.get("integrity") or "",
            "domain": a.get("domain") or "",
            "listener_id": a.get("listener_id"),
            "parent_id": a.get("parent_id") or a.get("parent_agent_id"),
        }
        subnets[sn].append(entry)
        agent_map[a["id"]] = entry

    edges = []
    for a in agents:
        aid = a["id"]
        lid = a.get("listener_id")
        if lid:
            edges.append({"from": aid[:8], "to": f"L-{lid}", "type": "listener"})
        parent = a.get("parent_id") or a.get("parent_agent_id")
        if parent and parent in agent_map:
            edges.append({"from": aid[:8], "to": parent[:8], "type": "p2p"})

    scan_by_target = defaultdict(lambda: {"ports": set(), "services": set()})
    for sr in scan_results:
        target = sr.get("target_ip") or ""
        if target:
            p = sr.get("port")
            if p:
                scan_by_target[target]["ports"].add(p)
            svc = sr.get("service")
            if svc:
                scan_by_target[target]["services"].add(svc)

    host_map = defaultdict(list)
    for h in network_hosts:
        ip = h.get("ip") or ""
        sn = _subnet(ip)
        host_map[sn].append({
            "ip": ip,
            "hostname": h.get("hostname") or "",
            "os": h.get("os") or "",
            "agent_id": h.get("agent_id"),
        })

    cross_subnet_edges = []
    for target, info in scan_by_target.items():
        target_sn = _subnet(target)
        for a in agents:
            a_sn = _subnet(a.get("ip") or "")
            if a_sn != target_sn:
                cross_subnet_edges.append({
                    "from": (a.get("id") or "")[:8],
                    "to": target,
                    "type": "scan",
                    "ports": sorted(info["ports"]),
                })

    listener_map = {}
    for l in listeners:
        listener_map[l["id"]] = {
            "id": l["id"],
            "name": l.get("name") or f"Listener {l['id']}",
            "host": l.get("host") or "",
            "port": l.get("port"),
            "scheme": l.get("scheme") or l.get("type") or "",
            "enabled": l.get("enabled"),
        }

    return {
        "subnets": dict(subnets),
        "agent_map": agent_map,
        "edges": edges,
        "cross_subnet_edges": cross_subnet_edges,
        "scan_by_target": {k: {"ports": sorted(v["ports"]), "services": sorted(v["services"])} for k, v in scan_by_target.items()},
        "host_map": dict(host_map),
        "listener_map": listener_map,
    }


def _ascii_topology(topo):
    lines = []
    lines.append("=" * 72)
    lines.append("  NETWORK TOPOLOGY — ASCII DIAGRAM")
    lines.append("=" * 72)

    for sn in sorted(topo["subnets"].keys()):
        agents = topo["subnets"][sn]
        lines.append("")
        lines.append(f"┌─ Subnet: {sn}.0/24 {'─' * (50 - len(sn))}┐")
        for i, a in enumerate(agents):
            is_last = i == len(agents) - 1
            prefix = "└" if is_last else "├"
            status_icon = "+" if a["status"] == "online" else "-"
            lines.append(
                f"│ {prefix}── [{status_icon}] {a['short_id']} "
                f"{a['hostname'][:20]:<20} {a['ip']:<16} {a['os']:<10} {a['status']}"
            )
        hosts = topo["host_map"].get(sn, [])
        if hosts:
            lines.append(f"│     └── Discovered hosts: {len(hosts)}")
            for h in hosts[:3]:
                lines.append(f"│         • {h['ip']} ({h['hostname'] or h['os'] or 'unknown'})")
            if len(hosts) > 3:
                lines.append(f"│         ... and {len(hosts) - 3} more")
        lines.append("└" + "─" * 60 + "┘")

    listeners_used = set()
    for e in topo["edges"]:
        if e["type"] == "listener":
            listeners_used.add(e["to"])

    if topo["listener_map"]:
        lines.append("")
        lines.append("Listeners:")
        for lid, l in sorted(topo["listener_map"].items()):
            marker = "*" if f"L-{lid}" in listeners_used else " "
            lines.append(f"  {marker} L-{lid}  {l['name']:<24} {l['scheme']}://{l['host']}:{l['port']}")

    if topo["cross_subnet_edges"]:
        lines.append("")
        lines.append("Cross-subnet scan connections:")
        for e in topo["cross_subnet_edges"]:
            ports_str = ",".join(str(p) for p in e["ports"][:5])
            lines.append(f"  {e['from']} --> {e['to']}  ports: {ports_str}")

    lines.append("")
    lines.append("=" * 72)
    return "\n".join(lines)


def generate_html(topo, title):
    total_agents = sum(len(v) for v in topo["subnets"].values())
    total_hosts = sum(len(v) for v in topo["host_map"].values())
    total_edges = len(topo["edges"]) + len(topo["cross_subnet_edges"])

    html = f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>{title}</title>
<style>
body {{ font-family: -apple-system, sans-serif; max-width: 1000px; margin: 0 auto; padding: 20px; color: #1a1a2e; background: #fafafa; }}
h1 {{ color: #0f3460; border-bottom: 3px solid #e94560; padding-bottom: 10px; }}
h2 {{ color: #16213e; margin-top: 28px; border-bottom: 1px solid #ddd; padding-bottom: 5px; }}
h3 {{ color: #0f3460; margin-top: 18px; }}
.grid {{ display: flex; flex-wrap: wrap; gap: 12px; margin: 15px 0; }}
.stat {{ flex: 1; min-width: 130px; background: #fff; border-left: 4px solid #0f3460; padding: 14px; border-radius: 6px; box-shadow: 0 1px 3px rgba(0,0,0,0.08); }}
.stat .num {{ font-size: 26px; font-weight: bold; color: #e94560; }}
.stat .label {{ font-size: 12px; color: #666; margin-top: 2px; }}
table {{ width: 100%; border-collapse: collapse; margin: 10px 0; font-size: 12px; }}
th {{ background: #0f3460; color: white; padding: 7px 10px; text-align: left; }}
td {{ border: 1px solid #ddd; padding: 5px 10px; }}
tr:nth-child(even) {{ background: #f8f9fa; }}
.badge {{ padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: bold; }}
.online {{ background: #d4edda; color: #155724; }}
.offline {{ background: #f8d7da; color: #721c24; }}
.stale {{ background: #fff3cd; color: #856404; }}
.subnet-box {{ background: #fff; border: 1px solid #e0e0e0; border-radius: 8px; padding: 16px; margin: 14px 0; box-shadow: 0 1px 2px rgba(0,0,0,0.05); }}
.subnet-title {{ font-size: 16px; font-weight: bold; color: #0f3460; margin-bottom: 8px; }}
.agent-row {{ display: flex; align-items: center; gap: 10px; padding: 5px 0; border-bottom: 1px solid #f0f0f0; font-size: 13px; }}
.agent-row:last-child {{ border-bottom: none; }}
.ascii-box {{ background: #1a1a2e; color: #00ff88; font-family: 'Courier New', monospace; padding: 16px; border-radius: 8px; overflow-x: auto; font-size: 12px; line-height: 1.5; white-space: pre; }}
.matrix-cell {{ text-align: center; font-size: 11px; }}
.reachable {{ background: #d4edda; }}
.unreachable {{ background: #f8f9fa; color: #ccc; }}
</style></head><body>
<h1>{title}</h1>
<p style="color:#666">Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}</p>

<h2>Summary</h2>
<div class="grid">
  <div class="stat"><div class="num">{len(topo['subnets'])}</div><div class="label">Subnets</div></div>
  <div class="stat"><div class="num">{total_agents}</div><div class="label">Agents</div></div>
  <div class="stat"><div class="num">{total_hosts}</div><div class="label">Discovered Hosts</div></div>
  <div class="stat"><div class="num">{len(topo['listener_map'])}</div><div class="label">Listeners</div></div>
  <div class="stat"><div class="num">{total_edges}</div><div class="label">Connections</div></div>
</div>

<h2>ASCII Topology</h2>
<div class="ascii-box">{_ascii_topology(topo)}</div>

<h2>Subnet Map</h2>"""

    for sn in sorted(topo["subnets"].keys()):
        agents = topo["subnets"][sn]
        hosts = topo["host_map"].get(sn, [])
        online = sum(1 for a in agents if a["status"] == "online")
        html += f"""
<div class="subnet-box">
  <div class="subnet-title">{sn}.0/24 — {len(agents)} agent(s), {len(hosts)} host(s), {online} online</div>
  <table><tr><th>ID</th><th>Hostname</th><th>User</th><th>IP</th><th>OS</th><th>Status</th><th>Integrity</th><th>Domain</th></tr>"""
        for a in agents:
            sc = a["status"] if a["status"] in ("online", "offline") else "stale"
            html += f"""
  <tr><td><code>{a['short_id']}</code></td><td>{a['hostname']}</td><td>{a['username']}</td>
  <td>{a['ip']}</td><td>{a['os']}</td><td><span class="badge {sc}">{a['status']}</span></td>
  <td>{a['integrity']}</td><td>{a['domain']}</td></tr>"""
        if hosts:
            html += """
  <tr><td colspan="8" style="background:#e8f4f8;font-weight:bold">Discovered Network Hosts</td></tr>"""
            for h in hosts:
                html += f"""
  <tr><td>—</td><td>{h['hostname'] or '—'}</td><td>—</td><td>{h['ip']}</td><td>{h['os']}</td><td colspan="3">discovered</td></tr>"""
        html += "\n</table>\n</div>"

    if topo["listener_map"]:
        html += "\n<h2>Listeners</h2>\n<table><tr><th>ID</th><th>Name</th><th>Address</th><th>Protocol</th><th>Agents</th></tr>"
        for lid, l in sorted(topo["listener_map"].items()):
            agent_count = sum(1 for a in topo["subnets"].values() for x in a if x.get("listener_id") == lid)
            html += f"\n<tr><td>L-{lid}</td><td>{l['name']}</td><td>{l['host']}:{l['port']}</td><td>{l['scheme']}</td><td>{agent_count}</td></tr>"
        html += "\n</table>"

    agent_ids = []
    for sn in sorted(topo["subnets"].keys()):
        for a in topo["subnets"][sn]:
            agent_ids.append(a["short_id"])
    if len(agent_ids) > 1:
        reachable = defaultdict(set)
        for e in topo["edges"]:
            reachable[e["from"]].add(e["to"])
        for e in topo["cross_subnet_edges"]:
            reachable[e["from"]].add(e["to"])

        html += f"\n\n<h2>Connectivity Matrix</h2>\n<table><tr><th></th>"
        for aid in agent_ids:
            html += f"<th class='matrix-cell'>{aid}</th>"
        html += "</tr>"
        for row_id in agent_ids:
            html += f"\n<tr><th>{row_id}</th>"
            for col_id in agent_ids:
                if row_id == col_id:
                    html += "<td class='matrix-cell' style='background:#0f3460;color:white'>—</td>"
                elif col_id in reachable.get(row_id, set()):
                    html += "<td class='matrix-cell reachable'>✓</td>"
                else:
                    html += "<td class='matrix-cell unreachable'>·</td>"
            html += "</tr>"
        html += "\n</table>"

    html += f"""

<h2>Network Statistics</h2>
<table><tr><th>Metric</th><th>Value</th></tr>"""
    for sn in sorted(topo["subnets"].keys()):
        agents = topo["subnets"][sn]
        hosts = topo["host_map"].get(sn, [])
        density = len(hosts) / 254 * 100 if hosts else 0
        html += f"\n<tr><td>{sn}.0/24 agents</td><td>{len(agents)}</td></tr>"
        html += f"\n<tr><td>{sn}.0/24 discovered hosts</td><td>{len(hosts)}</td></tr>"
        html += f"\n<tr><td>{sn}.0/24 host density</td><td>{density:.1f}%</td></tr>"
    html += f"""
<tr><td>Total unique subnets</td><td>{len(topo['subnets'])}</td></tr>
<tr><td>Total cross-subnet links</td><td>{len(topo['cross_subnet_edges'])}</td></tr>
</table>

</body></html>"""
    return html


def generate_markdown(topo, title):
    total_agents = sum(len(v) for v in topo["subnets"].values())
    total_hosts = sum(len(v) for v in topo["host_map"].values())

    lines = [
        f"# {title}",
        f"*Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}*",
        "",
        "## Summary",
        f"- **Subnets:** {len(topo['subnets'])}",
        f"- **Agents:** {total_agents}",
        f"- **Discovered Hosts:** {total_hosts}",
        f"- **Listeners:** {len(topo['listener_map'])}",
        "",
        "## ASCII Topology",
        "```",
        _ascii_topology(topo),
        "```",
        "",
        "## Subnet Map",
    ]
    for sn in sorted(topo["subnets"].keys()):
        agents = topo["subnets"][sn]
        hosts = topo["host_map"].get(sn, [])
        lines.append(f"\n### {sn}.0/24 ({len(agents)} agents, {len(hosts)} hosts)")
        lines.append("| ID | Hostname | IP | OS | Status |")
        lines.append("|---|---|---|---|---|")
        for a in agents:
            lines.append(f"| {a['short_id']} | {a['hostname']} | {a['ip']} | {a['os']} | {a['status']} |")
        if hosts:
            lines.append("\n**Discovered hosts:**")
            for h in hosts:
                lines.append(f"- {h['ip']} ({h['hostname'] or h['os'] or 'unknown'})")

    if topo["listener_map"]:
        lines += ["\n## Listeners", "| ID | Name | Address | Protocol |", "|---|---|---|---|"]
        for lid, l in sorted(topo["listener_map"].items()):
            lines.append(f"| L-{lid} | {l['name']} | {l['host']}:{l['port']} | {l['scheme']} |")

    lines += ["", "## Network Statistics"]
    for sn in sorted(topo["subnets"].keys()):
        agents = topo["subnets"][sn]
        hosts = topo["host_map"].get(sn, [])
        density = len(hosts) / 254 * 100 if hosts else 0
        lines.append(f"- **{sn}.0/24:** {len(agents)} agents, {len(hosts)} hosts ({density:.1f}% density)")
    lines.append(f"- **Total cross-subnet links:** {len(topo['cross_subnet_edges'])}")

    return "\n".join(lines)


def main():
    data_input = read_stdin()
    params = data_input.get("params", {})

    fmt = params.get("format", "html")
    title = params.get("title", "Network Topology Report")

    try:
        db = Database()
    except FileNotFoundError as e:
        print(json.dumps({"title": title, "format": fmt, "content": f"Error: {e}"}))
        return

    try:
        agents = db.all_agents()
        listeners = db.all_listeners()
        scan_results = db.all_scan_results()
        network_hosts = db.all_network_hosts()

        topo = build_topology(agents, listeners, scan_results, network_hosts)

        if fmt == "json":
            content = json.dumps({
                "title": title,
                "subnets": topo["subnets"],
                "listeners": topo["listener_map"],
                "edges": topo["edges"],
                "cross_subnet_edges": topo["cross_subnet_edges"],
                "scan_targets": topo["scan_by_target"],
                "network_hosts": topo["host_map"],
                "stats": {
                    "total_subnets": len(topo["subnets"]),
                    "total_agents": sum(len(v) for v in topo["subnets"].values()),
                    "total_hosts": sum(len(v) for v in topo["host_map"].values()),
                    "total_edges": len(topo["edges"]) + len(topo["cross_subnet_edges"]),
                },
            }, indent=2, default=str)
        elif fmt == "markdown":
            content = generate_markdown(topo, title)
        else:
            content = generate_html(topo, title)
            fmt = "html"

        write_report(title, content, fmt)
    finally:
        db.close()


if __name__ == "__main__":
    main()
