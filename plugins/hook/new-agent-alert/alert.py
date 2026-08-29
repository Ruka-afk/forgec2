#!/usr/bin/env python3
"""New Agent Alert hook plugin — alerts when new agents come online or reconnect."""

import json
import os
import sys
from datetime import datetime

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import read_stdin, write_result


def main():
    data = read_stdin()
    event = data.get("event", {})
    config = data.get("config", {})

    event_type = event.get("type", "")
    if event_type != "agent.connect":
        write_result(True, output=f"Ignored event: {event_type}", data={})
        return

    payload = event.get("payload", {})
    agent_id = event.get("agent_id", "")
    timestamp = event.get("timestamp", datetime.now().isoformat())

    is_new = payload.get("new", False)
    notify_new_only = config.get("notify_new_only", "true")
    if isinstance(notify_new_only, str):
        notify_new_only = notify_new_only.lower() == "true"

    if notify_new_only and not is_new:
        write_result(True, output="Skipped reconnect (notify_new_only=true)", data={})
        return

    hostname = payload.get("hostname", "unknown")
    ip = payload.get("ip", "unknown")
    os_info = payload.get("os", "unknown")
    username = payload.get("username", "unknown")
    integrity = payload.get("integrity", "unknown")
    domain = payload.get("domain", "")
    listener = payload.get("listener", "")
    last_beacon = payload.get("last_beacon", "")

    alert_level = "new" if is_new else "info"
    alert_type = "NEW AGENT" if is_new else "AGENT RECONNECT"

    output = f"{alert_type}: {hostname} ({os_info}) from {ip} | User: {username} | Integrity: {integrity}"
    if domain:
        output += f" | Domain: {domain}"

    context = {
        "agent_id": agent_id,
        "hostname": hostname,
        "ip": ip,
        "os": os_info,
        "username": username,
        "integrity": integrity,
        "domain": domain,
        "listener": listener,
        "is_new_agent": is_new,
        "alert_level": alert_level,
        "time_since_last_beacon": last_beacon,
        "timestamp": timestamp,
    }

    log_file = config.get("log_file", "")
    if log_file:
        try:
            with open(log_file, "a") as f:
                f.write(json.dumps(context, default=str) + "\n")
        except OSError as exc:
            print(json.dumps({"level": "error", "message": f"Plugin error: failed to write agent alert to log file '{log_file}': {exc}"}), file=sys.stderr)

    write_result(True, output=output, data=context)


if __name__ == "__main__":
    main()
