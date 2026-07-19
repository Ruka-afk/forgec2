#!/usr/bin/env python3
"""Persistence Scanner plugin — scans agents for persistence mechanisms and artifacts."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

PERSISTENCE_TASK_TYPES = (
    "persistence_add", "persistence_list", "persistence_remove",
    "scheduled_task", "cron", "startup",
)

WIN_PERSISTENCE_INDICATORS = {
    "run_key": ["reg add", "currentversion\\run", "currentversion\\runonce"],
    "scheduled_task": ["schtasks", "scheduledtask", "new-scheduledtask"],
    "service": ["new-service", "sc create", "sc config", "win32_service"],
    "startup_folder": ["startup", "shell:startup", "start menu"],
    "wmi": ["wmi", "ciminstance", "cimmethod", "__eventfilter", "__eventconsumer"],
}

LINUX_PERSISTENCE_INDICATORS = {
    "cron": ["crontab", "cron.d", "/etc/cron", "anacron"],
    "systemd": ["systemctl", "systemd", ".service", "/etc/systemd"],
    "shell_profile": [".bashrc", ".bash_profile", ".profile", ".zshrc"],
    "passwd": ["/etc/passwd", "useradd", "usermod"],
    "ssh_keys": ["authorized_keys", ".ssh/", "ssh-rsa"],
}


def analyze_agent(db, agent):
    """Analyze a single agent for persistence indicators."""
    agent_id = agent.get("id", "")
    os_type = (agent.get("os") or "").lower()
    hostname = agent.get("hostname", "unknown")

    mechanisms = []
    artifacts = []

    tasks = db.tasks_for_agent(agent_id)
    all_text = []

    for t in tasks:
        task_type = (t.get("type") or "").lower()
        command = (t.get("command") or "").lower()
        result_text = (t.get("result") or "").lower()
        all_text.append(command)
        all_text.append(result_text)

        if any(pt in task_type for pt in PERSISTENCE_TASK_TYPES):
            artifacts.append({
                "type": "forgec2_persistence_task",
                "task_id": t.get("id"),
                "task_type": t.get("type"),
                "command": (t.get("command") or "")[:200],
                "created_at": str(t.get("created_at", "")),
            })

            if task_type == "persistence_add":
                if "scheduled" in command or "schtasks" in command:
                    mechanisms.append("scheduled_task")
                elif "run" in command or "registry" in command:
                    mechanisms.append("run_key")
                elif "service" in command:
                    mechanisms.append("service")
                elif "cron" in command:
                    mechanisms.append("cron")
                else:
                    mechanisms.append("unknown_forgec2")

    combined = " ".join(all_text)
    indicators = WIN_PERSISTENCE_INDICATORS if "win" in os_type else LINUX_PERSISTENCE_INDICATORS

    for mechanism, keywords in indicators.items():
        for kw in keywords:
            if kw in combined and mechanism not in mechanisms:
                mechanisms.append(mechanism)
                break

    persistence_installed = bool(mechanisms)
    mechanisms = list(dict.fromkeys(mechanisms))

    return {
        "id": agent_id,
        "hostname": hostname,
        "os": agent.get("os", "unknown"),
        "status": agent.get("status", "unknown"),
        "persistence_installed": persistence_installed,
        "mechanisms": mechanisms,
        "artifacts": artifacts,
    }


def build_summary(agent_results):
    """Build summary statistics across all scanned agents."""
    total = len(agent_results)
    with_persistence = sum(1 for a in agent_results if a["persistence_installed"])
    without_persistence = total - with_persistence

    by_mechanism = {}
    for a in agent_results:
        for m in a["mechanisms"]:
            by_mechanism[m] = by_mechanism.get(m, 0) + 1

    return {
        "total": total,
        "with_persistence": with_persistence,
        "without_persistence": without_persistence,
        "by_mechanism": by_mechanism,
    }


def main():
    data = read_stdin()
    target_agent_id = data.get("agent_id") or data.get("params", {}).get("agent_id", "")

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        if target_agent_id:
            agent = db.agent_by_id(target_agent_id)
            if not agent:
                write_result(False, error=f"Agent {target_agent_id} not found")
                return
            agents = [agent]
        else:
            agents = db.all_agents()

        agent_results = [analyze_agent(db, a) for a in agents]
        summary = build_summary(agent_results)

        mechanism_str = ", ".join(
            f"{k}({v})" for k, v in sorted(summary["by_mechanism"].items(), key=lambda x: -x[1])
        ) if summary["by_mechanism"] else "none"

        output = (
            f"Scanned {summary['total']} agents | "
            f"{summary['with_persistence']} with persistence | "
            f"{len(summary['by_mechanism'])} mechanism types found\n"
            f"Mechanisms: {mechanism_str}"
        )

        write_result(
            True,
            output=output,
            data={
                "agents": agent_results,
                "summary": summary,
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
