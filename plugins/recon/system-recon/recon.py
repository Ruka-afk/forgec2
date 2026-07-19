#!/usr/bin/env python3
"""System Recon plugin — aggregates all agent, listener, and geo data."""

import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from lib.db import Database, read_stdin, write_result


def main():
    data = read_stdin()
    config = data.get("config", {})
    include_offline = config.get("include_offline", "true").lower() == "true"

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        agents = db.all_agents()
        listeners = db.all_listeners()

        if not include_offline:
            agents = [a for a in agents if a.get("status") == "online"]

        # Status breakdown
        status_counts = {}
        for a in agents:
            s = a.get("status", "unknown")
            status_counts[s] = status_counts.get(s, 0) + 1

        # OS breakdown
        os_counts = {}
        for a in agents:
            o = a.get("os", "unknown") or "unknown"
            os_counts[o] = os_counts.get(o, 0) + 1

        # Geo breakdown
        geo = {}
        for a in agents:
            country = a.get("country", "") or "Unknown"
            city = a.get("city", "") or "Unknown"
            key = f"{country}/{city}"
            geo[key] = geo.get(key, 0) + 1

        # Listener summary
        listener_summary = []
        for l in listeners:
            listener_summary.append({
                "id": l.get("id"),
                "name": l.get("name"),
                "scheme": l.get("scheme") or l.get("type"),
                "host": l.get("host"),
                "port": l.get("port"),
                "enabled": l.get("enabled"),
            })

        # Agent list (compact)
        agent_list = []
        for a in agents:
            agent_list.append({
                "id": a.get("id"),
                "hostname": a.get("hostname"),
                "username": a.get("username"),
                "os": a.get("os"),
                "ip": a.get("ip"),
                "public_ip": a.get("public_ip"),
                "country": a.get("country"),
                "status": a.get("status"),
                "last_seen": a.get("last_seen"),
                "pid": a.get("pid"),
                "process_name": a.get("process_name"),
                "integrity": a.get("integrity"),
                "elevated": a.get("elevated"),
                "domain": a.get("domain"),
                "listener_id": a.get("listener_id"),
            })

        output_lines = [
            f"Total agents: {len(agents)}",
            f"Online: {status_counts.get('online', 0)} | Offline: {status_counts.get('offline', 0)}",
            f"OS: {', '.join(f'{k}={v}' for k, v in sorted(os_counts.items(), key=lambda x: -x[1]))}",
            f"Locations: {len(geo)} unique",
            f"Listeners: {len(listeners)} ({sum(1 for l in listeners if l.get('enabled'))} active)",
        ]

        write_result(
            True,
            output="\n".join(output_lines),
            data={
                "total_agents": len(agents),
                "status_counts": status_counts,
                "os_counts": os_counts,
                "geo": geo,
                "listeners": listener_summary,
                "agents": agent_list,
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
