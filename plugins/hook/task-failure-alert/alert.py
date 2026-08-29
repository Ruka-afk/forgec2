#!/usr/bin/env python3
"""Task Failure Alert hook plugin — tracks consecutive task failures and alerts on thresholds."""

import json
import os
import sys
import time
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

# In-memory state keyed by agent_id: list of {timestamp, failed, task_type}
_agent_history: Dict[str, List[Dict[str, Any]]] = {}


def _parse_ts(ts_str: str) -> Optional[float]:
    """Parse an ISO timestamp string to a POSIX timestamp. Returns None on failure."""
    if not ts_str:
        return None
    try:
        dt = datetime.fromisoformat(ts_str.replace("Z", "+00:00"))
        return dt.timestamp()
    except (ValueError, TypeError):
        return None


def _prune_old(entries: List[Dict[str, Any]], now: float, window: float) -> List[Dict[str, Any]]:
    """Remove entries older than the time window."""
    cutoff = now - window
    return [e for e in entries if e["timestamp"] >= cutoff]


def _record_entry(agent_id: str, failed: bool, task_type: str, ts: float):
    """Append a task outcome entry for the agent."""
    _agent_history.setdefault(agent_id, []).append({
        "timestamp": ts,
        "failed": failed,
        "task_type": task_type,
    })


def _consecutive_failures(entries: List[Dict[str, Any]]) -> int:
    """Count trailing consecutive failures from the most recent entry backwards."""
    count = 0
    for e in reversed(entries):
        if e["failed"]:
            count += 1
        else:
            break
    return count


def _failure_types_by_consecutive(entries: List[Dict[str, Any]]) -> Optional[str]:
    """If the consecutive failures are all the same task_type, return it."""
    if not entries:
        return None
    types = set()
    for e in reversed(entries):
        if not e["failed"]:
            break
        types.add(e["task_type"])
    if len(types) == 1:
        return types.pop()
    return None


def _determine_alert_type(agent_id: str, consecutive: int, db: Database) -> str:
    """Determine whether failures indicate infrastructure, detection, or config issues."""
    all_agents = db.all_agents()
    if len(all_agents) <= 1:
        return "INFRASTRUCTURE"

    active_agents = [a for a in all_agents if a.get("status") != "dead"]
    failing_agents = 0
    for a in active_agents:
        aid = a.get("id", "")
        hist = _agent_history.get(aid, [])
        if _consecutive_failures(hist) >= consecutive:
            failing_agents += 1

    if failing_agents >= len(active_agents) and len(active_agents) > 1:
        return "INFRASTRUCTURE"
    return "DETECTION"


def _recommendation(alert_type: str, agent_id: str, task_type: Optional[str]) -> str:
    """Return a human-readable recommendation based on alert type."""
    if alert_type == "INFRASTRUCTURE":
        return "All agents failing — check C2 server logs, listener health, and network connectivity"
    if alert_type == "DETECTION":
        return f"Agent {agent_id[:8]} likely compromised by AV/EDR — check host defenses and rotate payload"
    if alert_type == "CONFIGURATION":
        return f"Tasks of type '{task_type}' failing consistently — review command parameters and agent compatibility"
    return "Review task history and agent status"


def main():
    data = read_stdin()
    event = data.get("event", {})
    config = data.get("config", {})

    event_type = event.get("type", "")
    if event_type != "task.completed":
        write_result(True, output=f"Ignored event: {event_type}", data={})
        return

    agent_id = event.get("agent_id", "")
    payload = event.get("payload", {})
    task_type = payload.get("type", payload.get("task_type", "unknown"))
    task_status = payload.get("status", "")
    raw_ts = event.get("timestamp", "")
    now = time.time()
    ts = _parse_ts(raw_ts) or now

    threshold = int(config.get("consecutive_threshold", 3))
    window = float(config.get("time_window_seconds", 300))
    log_file = config.get("log_file", "")

    failed = task_status in ("failed", "error", "timeout")

    _record_entry(agent_id, failed, task_type, ts)

    history = _agent_history.get(agent_id, [])
    pruned = _prune_old(history, now, window)
    _agent_history[agent_id] = pruned

    consecutive = _consecutive_failures(pruned)
    same_type = _failure_types_by_consecutive(pruned)

    summary = {
        "agent_id": agent_id,
        "consecutive_failures": consecutive,
        "task_type": task_type,
        "failed": failed,
        "threshold": threshold,
        "window_seconds": window,
        "evaluated_at": datetime.now(timezone.utc).isoformat(),
    }

    if consecutive < threshold:
        output = f"Task {task_status}: {task_type} on Agent {agent_id[:8]} (fail streak: {consecutive}/{threshold})"
        write_result(True, output=output, data=summary)
        return

    alert_type = _determine_alert_type(agent_id, consecutive, None)
    try:
        db = Database()
        alert_type = _determine_alert_type(agent_id, consecutive, db)
        db.close()
    except FileNotFoundError:
        alert_type = "INFRASTRUCTURE" if same_type is None else "CONFIGURATION"

    if same_type and alert_type == "DETECTION":
        alert_type = "CONFIGURATION"

    rec = _recommendation(alert_type, agent_id, same_type)

    summary["alert_type"] = alert_type
    summary["recommendation"] = rec
    summary["all_same_type"] = same_type is not None

    output = (
        f"FAILURE ALERT: Agent {agent_id[:8]} — "
        f"{consecutive} consecutive failures of type {task_type} | "
        f"Type: {alert_type} | Recommendation: {rec}"
    )

    if log_file:
        try:
            with open(log_file, "a") as f:
                f.write(json.dumps(summary, default=str) + "\n")
        except OSError as exc:
            print(json.dumps({"level": "error", "message": f"Plugin error: failed to write failure alert summary to log file '{log_file}': {exc}"}), file=sys.stderr)

    write_result(True, output=output, data=summary)


if __name__ == "__main__":
    main()
