#!/usr/bin/env python3
"""Network Mapper plugin — builds topology from agents, listeners, scan results, and discovered hosts."""

import json
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result


def main():
    data = read_stdin()
    config = data.get("config", {})
    max_display = int(config.get("max_display_hosts", 50))

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        agents = db.all_agents()
        listeners = db.all_listeners()
        scan_results = db.all_scan_results()
        network_hosts = db.all_network_hosts()

        # Build nodes
        nodes = []
        edges = []

        # Listener nodes
        for l in listeners:
            nodes.append({
                "id": f"listener-{l['id']}",
                "type": "listener",
                "label": l.get("name", f"Listener {l['id']}"),
                "host": l.get("host"),
                "port": l.get("port"),
                "scheme": l.get("scheme") or l.get("type"),
                "enabled": l.get("enabled"),
            })

        # Agent nodes
        agent_nodes = {}
        for a in agents:
            nid = f"agent-{a['id']}"
            agent_nodes[a["id"]] = nid
            nodes.append({
                "id": nid,
                "type": "agent",
                "label": a.get("hostname") or a["id"][:8],
                "ip": a.get("ip"),
                "public_ip": a.get("public_ip"),
                "os": a.get("os"),
                "status": a.get("status"),
                "country": a.get("country"),
                "elevated": a.get("elevated"),
            })
            # Edge: agent -> listener
            lid = a.get("listener_id")
            if lid:
                edges.append({
                    "from": nid,
                    "to": f"listener-{lid}",
                    "type": "connects_to",
                })
            # Edge: agent -> parent agent (P2P)
            parent = a.get("parent_id") or a.get("parent_agent_id")
            if parent and parent in agent_nodes:
                edges.append({
                    "from": nid,
                    "to": agent_nodes[parent],
                    "type": "p2p_child",
                })

        # Discovered host nodes
        for h in network_hosts[:max_display]:
            hid = f"host-{h['id']}"
            nodes.append({
                "id": hid,
                "type": "host",
                "label": h.get("hostname") or h.get("ip", "unknown"),
                "ip": h.get("ip"),
                "os": h.get("os"),
                "services": h.get("services"),
            })
            # Link to the agent that discovered it
            discoverer = h.get("agent_id")
            if discoverer and discoverer in agent_nodes:
                edges.append({
                    "from": agent_nodes[discoverer],
                    "to": hid,
                    "type": "discovered",
                })

        # Scan results summary
        scan_summary = {}
        for sr in scan_results:
            target = sr.get("target_ip", "unknown")
            if target not in scan_summary:
                scan_summary[target] = {"ports": [], "services": []}
            scan_summary[target]["ports"].append(sr.get("port"))
            svc = sr.get("service")
            if svc and svc not in scan_summary[target]["services"]:
                scan_summary[target]["services"].append(svc)

        # Stats
        stats = {
            "total_nodes": len(nodes),
            "total_edges": len(edges),
            "agents": len(agents),
            "listeners": len(listeners),
            "discovered_hosts": len(network_hosts),
            "scan_targets": len(scan_summary),
        }

        output_lines = [
            f"Network topology: {stats['total_nodes']} nodes, {stats['total_edges']} edges",
            f"Agents: {stats['agents']} | Listeners: {stats['listeners']}",
            f"Discovered hosts: {stats['discovered_hosts']} | Scan targets: {stats['scan_targets']}",
        ]

        write_result(
            True,
            output="\n".join(output_lines),
            data={
                "stats": stats,
                "nodes": nodes[:max_display * 2],
                "edges": edges[:max_display * 3],
                "scan_summary": scan_summary,
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
