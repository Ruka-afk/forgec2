#!/usr/bin/env python3
"""Agent Health Monitor hook plugin — tracks agent uptime and alerts on health issues."""

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
    agent_id = event.get("agent_id", "")
    payload = event.get("payload", {})
    timestamp = event.get("timestamp", datetime.now().isoformat())

    max_offline = int(config.get("max_offline_seconds", 600))
    log_file = config.get("log_file", "")

    metrics = {
        "event_type": event_type,
        "agent_id": agent_id,
        "timestamp": timestamp,
        "hostname": payload.get("hostname", ""),
        "ip": payload.get("ip", ""),
        "os": payload.get("os", ""),
    }

    if event_type == "agent.connect":
        is_new = payload.get("new", False)
        metrics["is_new_agent"] = is_new
        output = f"Agent connected: {agent_id[:8]} ({metrics['hostname']})"
        if is_new:
            output += " [NEW AGENT]"

    elif event_type == "agent.disconnect":
        offline_secs = payload.get("offline_for_seconds", 0)
        metrics["offline_seconds"] = offline_secs
        if offline_secs > max_offline:
            metrics["alert"] = "extended_offline"
            output = f"ALERT: Agent {agent_id[:8]} offline for {offline_secs}s (threshold: {max_offline}s)"
        else:
            output = f"Agent disconnected: {agent_id[:8]} (offline {offline_secs}s)"
    else:
        output = f"Health monitor: unhandled event {event_type}"

    # Optionally log to file
    if log_file:
        try:
            with open(log_file, "a") as f:
                f.write(json.dumps(metrics) + "\n")
        except OSError as exc:
            print(json.dumps({"level": "error", "message": f"Plugin error: failed to write health metrics to log file '{log_file}': {exc}"}), file=sys.stderr)

    write_result(True, output=output, data=metrics)


if __name__ == "__main__":
    main()
