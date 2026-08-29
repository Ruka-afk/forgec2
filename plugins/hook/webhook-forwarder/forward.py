#!/usr/bin/env python3
"""Webhook Forwarder hook plugin — forwards ForgeC2 events to external webhooks."""

import json
import os
import sys
import urllib.request
import urllib.error
from datetime import datetime

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import read_stdin, write_result


def _format_payload(event_type, agent_id, detail, webhook_type):
    if webhook_type == "slack":
        return json.dumps({
            "text": f"Event: {event_type}\nAgent: {agent_id}\nDetail: {detail}",
        })
    if webhook_type == "discord":
        return json.dumps({
            "content": f"Event: {event_type}\nAgent: {agent_id}\nDetail: {detail}",
        })
    return json.dumps({
        "event_type": event_type,
        "agent_id": agent_id,
        "detail": detail,
    })


def main():
    data = read_stdin()
    event = data.get("event", {})
    config = data.get("config", {})

    event_type = event.get("type", "")
    agent_id = event.get("agent_id", "")
    payload = event.get("payload", {})

    webhook_url = config.get("webhook_url", "")
    if not webhook_url:
        write_result(False, output="No webhook_url configured", error="missing config")
        return

    webhook_type = config.get("webhook_type", "generic")

    events_filter = config.get("events_filter", "")
    if events_filter:
        allowed = [e.strip() for e in events_filter.split(",") if e.strip()]
        if allowed and event_type not in allowed:
            output = f"Forwarded {event_type} to {webhook_type} | Status: skipped"
            write_result(True, output=output, data={
                "event_type": event_type,
                "agent_id": agent_id,
                "webhook_type": webhook_type,
                "status": "skipped",
                "detail": f"Filtered out (not in events_filter)",
            })
            return

    detail = json.dumps(payload, default=str) if payload else ""

    body = _format_payload(event_type, agent_id, detail, webhook_type)
    req = urllib.request.Request(
        webhook_url,
        data=body.encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    status = "sent"
    detail_msg = ""
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            detail_msg = f"HTTP {resp.status}"
    except urllib.error.URLError as e:
        status = "failed"
        detail_msg = f"URLError: {e.reason}"
    except Exception as e:
        status = "failed"
        detail_msg = str(e)

    log_file = config.get("log_file", "")
    if log_file:
        try:
            log_entry = {
                "event_type": event_type,
                "agent_id": agent_id,
                "webhook_type": webhook_type,
                "status": status,
                "detail": detail_msg,
                "timestamp": datetime.now().isoformat(),
            }
            with open(log_file, "a") as f:
                f.write(json.dumps(log_entry) + "\n")
        except OSError as exc:
            print(json.dumps({"level": "error", "message": f"Plugin error: failed to write webhook log entry to log file '{log_file}': {exc}"}), file=sys.stderr)

    output = f"Forwarded {event_type} to {webhook_type} | Status: {status}"
    if detail_msg:
        output += f" ({detail_msg})"

    write_result(status == "sent", output=output, data={
        "event_type": event_type,
        "agent_id": agent_id,
        "webhook_type": webhook_type,
        "status": status,
        "detail": detail_msg,
    })


if __name__ == "__main__":
    main()
