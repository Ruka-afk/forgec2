#!/usr/bin/env python3
"""Credential Rotation hook plugin — monitors credential age and alerts when rotation is recommended."""

import json
import os
import sys
from datetime import datetime, timezone

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

CREDENTIAL_TASK_KEYWORDS = ("cred", "hash", "mimikatz", "browser", "kerberoast")


def is_credential_task(task_type: str) -> bool:
    t = task_type.lower()
    return any(kw in t for kw in CREDENTIAL_TASK_KEYWORDS)


def main():
    data = read_stdin()
    event = data.get("event", {})
    config = data.get("config", {})

    event_type = event.get("type", "")
    if event_type != "task.completed":
        write_result(True, output=f"Ignored event: {event_type}", data={})
        return

    payload = event.get("payload", {})
    task_type = payload.get("type", payload.get("task_type", ""))
    agent_id = event.get("agent_id", "")

    if not is_credential_task(task_type):
        write_result(True, output=f"Ignored non-credential task: {task_type}", data={})
        return

    try:
        db = Database()
    except FileNotFoundError as exc:
        write_result(False, error=str(exc))
        return

    try:
        all_creds = db.all_credentials()
        agent_creds = [c for c in all_creds if c.get("agent_id") == agent_id]
    finally:
        db.close()

    max_age = int(config.get("max_age_days", 30))
    warning_age = int(config.get("warning_age_days", 14))

    now = datetime.now(timezone.utc)
    need_rotation = 0
    warning_count = 0
    oldest_days = 0

    for cred in agent_creds:
        created = cred.get("created_at")
        if not created:
            continue
        if isinstance(created, str):
            try:
                created = datetime.fromisoformat(created)
            except ValueError:
                continue
        if created.tzinfo is None:
            created = created.replace(tzinfo=timezone.utc)

        age_days = (now - created).days
        if age_days > oldest_days:
            oldest_days = age_days

        if age_days > max_age:
            need_rotation += 1
        elif age_days > warning_age:
            warning_count += 1

    total_checked = len(agent_creds)

    output_parts = [
        f"Credentials checked: {total_checked}",
        f"Need rotation: {need_rotation}",
        f"Approaching limit: {warning_count}",
        f"Oldest credential: {oldest_days} days",
    ]

    output = " | ".join(output_parts)

    if need_rotation > 0:
        output = (
            f"ROTATION ALERT: Agent {agent_id} — "
            f"{need_rotation} credentials older than {max_age} days | "
            f"Consider re-harvesting"
        )

    result_data = {
        "agent_id": agent_id,
        "credentials_checked": total_checked,
        "need_rotation": need_rotation,
        "warning": warning_count,
        "oldest_credential_days": oldest_days,
    }

    log_file = config.get("log_file", "")
    if log_file:
        try:
            with open(log_file, "a") as f:
                f.write(json.dumps(result_data, default=str) + "\n")
        except OSError as exc:
            print(json.dumps({"level": "error", "message": f"Plugin error: failed to write rotation results to log file '{log_file}': {exc}"}), file=sys.stderr)

    write_result(True, output=output, data=result_data)


if __name__ == "__main__":
    main()
