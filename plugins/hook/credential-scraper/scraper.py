#!/usr/bin/env python3
"""Credential Scraper hook plugin — extracts credentials from completed task results."""

import json
import os
import re
import sys
from datetime import datetime

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import read_stdin, write_result

PATTERNS = {
    "email": re.compile(r"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}"),
    "password": re.compile(
        r"(?:password|passwd|pwd)[=:]\s*(\S+)", re.IGNORECASE
    ),
    "ntlm_hash": re.compile(r"NTLM:([a-fA-F0-9]{32})"),
    "lm_hash": re.compile(r"LM:([a-fA-F0-9]{32})"),
    "hash_pair": re.compile(r"([a-fA-F0-9]{32}):([a-fA-F0-9]{32})"),
    "api_key": re.compile(
        r"api[_-]?key[=:]\s*['\"]?([a-zA-Z0-9_-]{20,})['\"]?", re.IGNORECASE
    ),
    "credential_pair": re.compile(r"([a-zA-Z0-9._-]+):([a-zA-Z0-9._!@#$%^&*]+)@"),
    "kerberos": re.compile(r"(krbtgt/.*|TGT.*krbtgt)", re.IGNORECASE),
}


def extract_credentials(text, task_id, task_type):
    found = []

    for label, pattern in PATTERNS.items():
        for match in pattern.finditer(text):
            value = match.group(0) if label in ("email", "kerberos", "hash_pair") else match.group(1)
            found.append({
                "type": label,
                "value": value,
                "source_task_id": task_id,
                "task_type": task_type,
            })

    return found


def main():
    data = read_stdin()
    event = data.get("event", {})
    config = data.get("config", {})

    event_type = event.get("type", "")
    if event_type not in ("task.completed", "task.result"):
        write_result(True, output=f"Ignored event: {event_type}", data={})
        return

    payload = event.get("payload", {})
    result_text = payload.get("result", "")
    task_type = payload.get("type", payload.get("task_type", "unknown"))
    task_id = event.get("task_id", payload.get("task_id", ""))
    agent_id = event.get("agent_id", "")

    if not result_text:
        write_result(True, output="No result text to scrape", data={})
        return

    credentials = extract_credentials(result_text, task_id, task_type)

    by_type = {}
    for cred in credentials:
        by_type[cred["type"]] = by_type.get(cred["type"], 0) + 1

    parts = []
    for t in ("email", "password", "hash", "ntlm_hash", "lm_hash", "hash_pair",
              "api_key", "credential_pair", "kerberos"):
        count = by_type.get(t, 0)
        if count:
            parts.append(f"{count} {t}")

    summary = {
        "total_found": len(credentials),
        "by_type": by_type,
        "task_id": task_id,
        "agent_id": agent_id,
    }

    output = f"Scraped task result | Found {len(credentials)} credentials ({', '.join(parts) or 'none'})"

    log_file = config.get("log_file", "")
    if log_file:
        try:
            with open(log_file, "a") as f:
                for cred in credentials:
                    f.write(json.dumps(cred, default=str) + "\n")
        except OSError:
            pass

    write_result(True, output=output, data={"found": credentials, "summary": summary})


if __name__ == "__main__":
    main()
