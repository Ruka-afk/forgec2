#!/usr/bin/env python3
"""Burn Detector hook plugin — detects agent burn indicators and scores compromise risk."""

import json
import os
import re
import sys
from datetime import datetime

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import read_stdin, write_result

BURN_INDICATORS = [
    {
        "pattern": re.compile(r"access denied|permission denied|not authorized", re.IGNORECASE),
        "label": "access_denied",
        "score": 2,
    },
    {
        "pattern": re.compile(r"connection refused|network unreachable|timed? ?out", re.IGNORECASE),
        "label": "network_error",
        "score": 2,
    },
    {
        "pattern": re.compile(r"blocked by|quarantined|detected|removed", re.IGNORECASE),
        "label": "av_detected",
        "score": 5,
    },
    {
        "pattern": re.compile(r"firewall|waf|ids.?ips|intrusion", re.IGNORECASE),
        "label": "defense_blocking",
        "score": 4,
    },
    {
        "pattern": re.compile(r"file not found|no such file|does not exist", re.IGNORECASE),
        "label": "file_missing",
        "score": 1,
    },
    {
        "pattern": re.compile(r"windows defender|msmpeng|antimalware|clamav|sophos|crowdstrike|sentinel", re.IGNORECASE),
        "label": "av_product",
        "score": 3,
    },
    {
        "pattern": re.compile(r"process.?kill|terminated|force.?quit|killed", re.IGNORECASE),
        "label": "process_killed",
        "score": 4,
    },
    {
        "pattern": re.compile(r"unable to load|load failed|module not found|dll.?inject.?fail", re.IGNORECASE),
        "label": "load_failure",
        "score": 3,
    },
    {
        "pattern": re.compile(r"sign.?atur|hash.?mismatch|tamper|integrity.?fail", re.IGNORECASE),
        "label": "tamper_detected",
        "score": 5,
    },
    {
        "pattern": re.compile(r"sandbox|analysis.?environment|vm.?detect|debugger.?found", re.IGNORECASE),
        "label": "analysis_env",
        "score": 3,
    },
]

CRITICAL_PATHS = [
    r"C:\\Windows\\System32",
    r"C:\\Windows\\SysWOW64",
    r"C:\\Program Files",
    r"C:\\ProgramData",
]


def score_task_result(result_text):
    indicators = []
    score = 0

    for ind in BURN_INDICATORS:
        matches = ind["pattern"].findall(result_text)
        if matches:
            count = len(matches)
            point = min(ind["score"] * count, 10)
            indicators.append({
                "label": ind["label"],
                "occurrences": count,
                "points": point,
            })
            score += point

    if any(cp.lower() in result_text.lower() for cp in CRITICAL_PATHS):
        indicators.append({
            "label": "critical_path",
            "occurrences": 1,
            "points": 1,
        })
        score += 1

    return min(score, 10), indicators


def score_disconnect(payload):
    indicators = []
    score = 0

    offline_secs = payload.get("offline_for_seconds", 0)
    last_result = payload.get("last_result", "")

    if last_result and any(
        ind["pattern"].search(last_result) for ind in BURN_INDICATORS
    ):
        _, last_indicators = score_task_result(last_result)
        indicators.extend(last_indicators)
        score += sum(i["points"] for i in last_indicators)

    if offline_secs > 3600:
        indicators.append({
            "label": "extended_offline",
            "occurrences": 1,
            "points": 2,
        })
        score += 2

    return min(score, 10), indicators


def score_connect(payload):
    indicators = []
    score = 0

    was_flagged = payload.get("previously_flagged", False)
    if was_flagged:
        indicators.append({
            "label": "previously_flagged",
            "occurrences": 1,
            "points": 3,
        })
        score += 3

    return min(score, 10), indicators


def risk_level(score, threshold):
    if score >= threshold:
        return "likely_burned"
    elif score >= max(1, threshold // 2):
        return "suspicious"
    return "safe"


def build_recommendation(level, score):
    if level == "likely_burned":
        return (
            f"URGENT: Agent scored {score}/10. "
            "Consider immediate burn — destroy artifact, rotate C2 infrastructure, "
            "and pivot to new beacon. Avoid further tasking on this host."
        )
    elif level == "suspicious":
        return (
            f"Agent scored {score}/10 with suspicious indicators. "
            "Reduce task frequency, avoid sensitive operations, "
            "and monitor closely for confirmation of detection."
        )
    return "No burn indicators detected. Agent appears clean."


def main():
    data = read_stdin()
    event = data.get("event", {})
    config = data.get("config", {})

    event_type = event.get("type", "")
    agent_id = event.get("agent_id", "")
    payload = event.get("payload", {})
    timestamp = event.get("timestamp", datetime.now().isoformat())
    threshold = int(config.get("alert_threshold", 3))
    log_file = config.get("log_file", "")

    if event_type == "task.completed":
        result_text = payload.get("result", "")
        burn_score, indicators = score_task_result(result_text)
    elif event_type == "agent.disconnect":
        burn_score, indicators = score_disconnect(payload)
    elif event_type == "agent.connect":
        burn_score, indicators = score_connect(payload)
    else:
        burn_score, indicators = 0, []

    level = risk_level(burn_score, threshold)
    recommendation = build_recommendation(level, burn_score)

    data_out = {
        "agent_id": agent_id,
        "event_type": event_type,
        "burn_score": burn_score,
        "indicators": indicators,
        "risk_level": level,
        "recommendation": recommendation,
        "timestamp": timestamp,
    }

    if level == "likely_burned":
        labels = [i["label"] for i in indicators]
        output = f"BURN ALERT: Agent {agent_id[:8]} scored {burn_score}/10 — indicators: [{', '.join(labels)}]"
    elif level == "suspicious":
        labels = [i["label"] for i in indicators]
        output = f"Burn warning: Agent {agent_id[:8]} scored {burn_score}/10 — indicators: [{', '.join(labels)}]"
    else:
        output = f"Burn check: Agent {agent_id[:8]} scored {burn_score}/10 — no risk detected"

    if log_file:
        try:
            with open(log_file, "a") as f:
                f.write(json.dumps(data_out, default=str) + "\n")
        except OSError:
            pass

    write_result(True, output=output, data=data_out)


if __name__ == "__main__":
    main()
