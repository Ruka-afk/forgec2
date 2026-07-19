#!/usr/bin/env python3
"""Security Event Logger hook plugin — records all security events for audit trail."""

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
    verbose = config.get("verbose", "true").lower() == "true"

    record = {
        "timestamp": timestamp,
        "event_type": event_type,
        "agent_id": agent_id,
        "user_id": event.get("user_id"),
        "severity": "info",
    }

    if event_type == "agent.connect":
        record["detail"] = f"Agent connected from {payload.get('ip', 'unknown')} ({payload.get('hostname', '')})"
        record["severity"] = "info"
        if payload.get("new"):
            record["severity"] = "notice"
            record["detail"] += " [NEW]"

    elif event_type == "agent.disconnect":
        record["detail"] = f"Agent disconnected (offline {payload.get('offline_for_seconds', 0)}s)"
        record["severity"] = "warning"

    elif event_type == "task.created":
        record["detail"] = f"Task created: type={payload.get('task_type', '?')}, cmd={str(payload.get('command', ''))[:60]}"
        record["severity"] = "info"

    elif event_type == "task.completed":
        record["detail"] = f"Task completed: type={payload.get('task_type', '?')}, status={payload.get('status', '?')}"
        if payload.get("error"):
            record["severity"] = "warning"
            record["detail"] += f", error={payload['error'][:60]}"

    elif event_type == "user.login":
        record["detail"] = f"User login: {payload.get('username', '?')} from {payload.get('ip', 'unknown')}"
        record["severity"] = "info"
        record["user"] = payload.get("username")

    elif event_type == "user.logout":
        record["detail"] = f"User logout: {payload.get('username', '?')}"
        record["severity"] = "info"
        record["user"] = payload.get("username")

    else:
        record["detail"] = f"Unknown event: {event_type}"
        record["severity"] = "debug"

    # Optionally write to log file
    log_file = config.get("log_file", "")
    if log_file:
        try:
            with open(log_file, "a") as f:
                f.write(json.dumps(record, default=str) + "\n")
        except OSError:
            pass

    output = f"[{record['severity'].upper()}] {record['detail']}"
    write_result(True, output=output, data=record)


if __name__ == "__main__":
    main()
