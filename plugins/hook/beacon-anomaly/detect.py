#!/usr/bin/env python3
"""Beacon Anomaly hook plugin — detects anomalous beacon timing, payload size, and jitter patterns."""

import json
import math
import os
import sys
from datetime import datetime

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result


def _safe_stdev(values):
    """Compute sample standard deviation, returns 0 if insufficient data."""
    if len(values) < 2:
        return 0.0
    mean = sum(values) / len(values)
    variance = sum((v - mean) ** 2 for v in values) / (len(values) - 1)
    return math.sqrt(variance)


def _coefficient_of_variation(values):
    """Compute CV (stdev/mean). Returns 0 if mean is 0 or insufficient data."""
    if len(values) < 2:
        return 0.0
    mean = sum(values) / len(values)
    if mean == 0:
        return 0.0
    return _safe_stdev(values) / mean


def _risk_from_anomalies(anomalies):
    """Derive overall risk level from anomaly severities."""
    severities = [a["severity"] for a in anomalies]
    if "high" in severities:
        return "high"
    if "medium" in severities:
        return "elevated"
    return "normal"


def _db_history(db, agent_id):
    """Retrieve recent beacon timing and payload size history for an agent."""
    timings = []
    sizes = []
    try:
        tasks = db.tasks_for_agent(agent_id)
        completed = [t for t in tasks if t.get("status") in ("completed", "success")]
        completed.sort(key=lambda t: t.get("created_at", ""))
        for i, t in enumerate(completed):
            payload = t.get("result", "")
            sizes.append(len(payload) if isinstance(payload, str) else 0)
            if i > 0:
                t1 = t.get("created_at", "")
                t0 = completed[i - 1].get("created_at", "")
                if t1 and t0:
                    try:
                        d1 = datetime.fromisoformat(t1.replace("Z", "+00:00"))
                        d0 = datetime.fromisoformat(t0.replace("Z", "+00:00"))
                        delta = (d1 - d0).total_seconds()
                        if delta > 0:
                            timings.append(delta)
                    except (ValueError, TypeError) as exc:
                        print(json.dumps({"level": "error", "message": f"Plugin error: invalid beacon timestamp pair: {exc}"}), file=sys.stderr)
    except Exception as exc:
        print(json.dumps({"level": "error", "message": f"Plugin error: failed to load beacon history for agent '{agent_id}': {exc}"}), file=sys.stderr)
    return timings, sizes


def check_timing(payload, config, db, agent_id):
    """Check if beacon timing deviates from configured interval."""
    anomalies = []
    configured_interval = payload.get("sleep_seconds", payload.get("interval", 0))
    if configured_interval <= 0:
        return anomalies

    timing_threshold = float(config.get("timing_threshold", 3.0))
    actual_interval = payload.get("actual_interval", 0)

    if actual_interval > 0:
        deviation = abs(actual_interval - configured_interval) / configured_interval
        if deviation > 1.0:
            anomalies.append({
                "type": "timing_deviation",
                "severity": "high",
                "detail": f"Beacon interval {actual_interval:.1f}s vs configured {configured_interval:.1f}s ({deviation*100:.0f}% deviation)",
            })
        elif deviation > 0.5:
            anomalies.append({
                "type": "timing_drift",
                "severity": "medium",
                "detail": f"Beacon interval {actual_interval:.1f}s vs configured {configured_interval:.1f}s ({deviation*100:.0f}% deviation)",
            })

    timings, _ = _db_history(db, agent_id)
    if len(timings) >= 3:
        stdev = _safe_stdev(timings)
        mean = sum(timings) / len(timings)
        if mean > 0 and stdev / mean < 0.01:
            anomalies.append({
                "type": "timing_regularity",
                "severity": "medium",
                "detail": f"Beacons too regular (CV={stdev/mean:.4f}) — possible automated/programmatic control",
            })

    return anomalies


def check_payload_size(payload, config, db, agent_id):
    """Check if payload size deviates from historical baseline."""
    anomalies = []
    size_threshold = float(config.get("size_threshold", 5.0))

    _, sizes = _db_history(db, agent_id)
    if len(sizes) < 5:
        return anomalies

    current_size = payload.get("payload_size", 0)
    if current_size <= 0:
        return anomalies

    mean_size = sum(sizes) / len(sizes)
    stdev_size = _safe_stdev(sizes)
    if stdev_size == 0:
        return anomalies

    z_score = abs(current_size - mean_size) / stdev_size
    if z_score > size_threshold:
        anomalies.append({
            "type": "payload_size_anomaly",
            "severity": "high",
            "detail": f"Payload size {current_size}B deviates {z_score:.1f}σ from baseline (μ={mean_size:.0f}B, σ={stdev_size:.0f}B)",
        })
    elif z_score > size_threshold * 0.6:
        anomalies.append({
            "type": "payload_size_drift",
            "severity": "low",
            "detail": f"Payload size {current_size}B shows mild drift ({z_score:.1f}σ from baseline)",
        })

    return anomalies


def check_jitter(payload, config):
    """Check if actual jitter deviates significantly from configured jitter."""
    anomalies = []
    configured_jitter = payload.get("jitter_percent", payload.get("jitter", 0))
    actual_jitter = payload.get("actual_jitter", -1)
    if actual_jitter < 0 or configured_jitter <= 0:
        return anomalies

    ratio = actual_jitter / configured_jitter if configured_jitter else 0
    if ratio < 0.1:
        anomalies.append({
            "type": "jitter_too_low",
            "severity": "high",
            "detail": f"Actual jitter {actual_jitter:.1f}% far below configured {configured_jitter}% — possible sleepless/manual execution",
        })
    elif ratio > 3.0:
        anomalies.append({
            "type": "jitter_too_high",
            "severity": "medium",
            "detail": f"Actual jitter {actual_jitter:.1f}% far above configured {configured_jitter}% — possible network manipulation",
        })

    return anomalies


def handle_beacon(data, db, config):
    """Analyze a beacon.received event for anomalies."""
    event = data.get("event", {})
    payload = event.get("payload", {})
    agent_id = event.get("agent_id", "")

    anomalies = []
    anomalies.extend(check_timing(payload, config, db, agent_id))
    anomalies.extend(check_payload_size(payload, config, db, agent_id))
    anomalies.extend(check_jitter(payload, config))

    overall_risk = _risk_from_anomalies(anomalies)
    summary = {
        "agent_id": agent_id,
        "anomalies": anomalies,
        "overall_risk": overall_risk,
        "checked_at": datetime.now().isoformat(),
    }

    if anomalies:
        types = ", ".join(a["type"] for a in anomalies)
        output = f"ANOMALY: Agent {agent_id[:8]} — {types}"
    else:
        output = f"Beacon normal for Agent {agent_id[:8]}"

    return output, summary


def handle_connect(data, db, config):
    """Analyze agent.connect for timing consistency."""
    event = data.get("event", {})
    payload = event.get("payload", {})
    agent_id = event.get("agent_id", "")

    anomalies = check_timing(payload, config, db, agent_id)

    overall_risk = _risk_from_anomalies(anomalies)
    summary = {
        "agent_id": agent_id,
        "anomalies": anomalies,
        "overall_risk": overall_risk,
        "event_type": "agent.connect",
    }

    if anomalies:
        types = ", ".join(a["type"] for a in anomalies)
        output = f"ANOMALY: Agent {agent_id[:8]} connect — {types}"
    else:
        output = f"Agent connected normally: {agent_id[:8]}"

    return output, summary


def handle_disconnect(data, db, config):
    """Analyze agent.disconnect for unexpected patterns."""
    event = data.get("event", {})
    payload = event.get("payload", {})
    agent_id = event.get("agent_id", "")

    anomalies = []
    offline_secs = payload.get("offline_for_seconds", 0)

    configured_interval = payload.get("sleep_seconds", payload.get("interval", 0))
    if configured_interval > 0 and offline_secs > 0:
        if offline_secs < configured_interval * 0.3:
            anomalies.append({
                "type": "early_disconnect",
                "severity": "medium",
                "detail": f"Agent disconnected after {offline_secs}s — early relative to {configured_interval}s interval",
            })

    overall_risk = _risk_from_anomalies(anomalies)
    summary = {
        "agent_id": agent_id,
        "anomalies": anomalies,
        "overall_risk": overall_risk,
        "event_type": "agent.disconnect",
    }

    if anomalies:
        types = ", ".join(a["type"] for a in anomalies)
        output = f"ANOMALY: Agent {agent_id[:8]} disconnect — {types}"
    else:
        output = f"Agent disconnected normally: {agent_id[:8]}"

    return output, summary


def main():
    data = read_stdin()
    event = data.get("event", {})
    config = data.get("config", {})

    event_type = event.get("type", "")
    log_file = config.get("log_file", "")

    try:
        db = Database()
    except FileNotFoundError:
        write_result(False, error="Database not found", data={})
        return

    try:
        if event_type == "beacon.received":
            output, summary = handle_beacon(data, db, config)
        elif event_type == "agent.connect":
            output, summary = handle_connect(data, db, config)
        elif event_type == "agent.disconnect":
            output, summary = handle_disconnect(data, db, config)
        else:
            output = f"Ignored event: {event_type}"
            summary = {"event_type": event_type}
    finally:
        db.close()

    if log_file and summary.get("anomalies"):
        try:
            with open(log_file, "a") as f:
                f.write(json.dumps(summary, default=str) + "\n")
        except OSError as exc:
            print(json.dumps({"level": "error", "message": f"Plugin error: failed to write anomaly summary to log file '{log_file}': {exc}"}), file=sys.stderr)

    write_result(True, output=output, data=summary)


if __name__ == "__main__":
    main()
