#!/usr/bin/env python3
"""Cleanup Tracker plugin — analyzes artifacts that may need cleanup for OPSEC."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result


def main():
    data = read_stdin()
    agent_id = data.get("agent_id", "")
    config = data.get("config", {})

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        artifacts = []
        recommendations = []

        # 1. Check for completed tasks with sensitive commands
        tasks = db.tasks_for_agent(agent_id) if agent_id else db.all_tasks()
        sensitive_keywords = [
            "mimikatz", "sekurlsa", "kerberos::list", "lsadump",
            "Invoke-Mimikatz", "Invoke-WebRequest", "Invoke-Expression",
            "DownloadString", "DownloadFile", "certutil", "bitsadmin",
            "Invoke-CimMethod", "Get-WmiObject", "Win32_Process",
            "New-ScheduledTask", "Reg Add", "schtasks",
        ]
        for t in tasks:
            cmd = (t.get("command") or "").lower()
            for kw in sensitive_keywords:
                if kw.lower() in cmd:
                    artifacts.append({
                        "type": "sensitive_command",
                        "task_id": t.get("id"),
                        "agent_id": t.get("agent_id"),
                        "command": (t.get("command") or "")[:120],
                        "keyword": kw,
                        "created_at": str(t.get("created_at")),
                    })
                    break

        # 2. Check for tasks with file operations (download/upload traces)
        file_tasks = [t for t in tasks if t.get("type") in ("upload", "download", "read", "delete")]
        for t in file_tasks[:50]:
            artifacts.append({
                "type": "file_operation",
                "task_id": t.get("id"),
                "agent_id": t.get("agent_id"),
                "operation": t.get("type"),
                "path": t.get("path") or t.get("command"),
                "created_at": str(t.get("created_at")),
            })

        # 3. Audit log analysis
        if config.get("check_audit_logs", "true") == "true":
            audit_logs = db.all_audit_logs()
            op_actions = [a for a in audit_logs if a.get("action") in (
                "shell", "command", "screenshot", "upload", "download",
                "file_delete", "file_read", "kill_process",
            )]
            for a in op_actions[:50]:
                artifacts.append({
                    "type": "audit_log",
                    "log_id": a.get("id"),
                    "user": a.get("user"),
                    "action": a.get("action"),
                    "agent_id": a.get("agent_id"),
                    "ip": a.get("ip"),
                    "created_at": str(a.get("created_at")),
                })

        # 4. Check for active SOCKS sessions (network traces)
        socks = db.all_socks_sessions()
        active_socks = [s for s in socks if s.get("status") == "active"]
        for s in active_socks:
            artifacts.append({
                "type": "active_socks",
                "session_id": s.get("id"),
                "agent_id": s.get("agent_id"),
                "listen_port": s.get("listen_port"),
                "bytes_transferred": (s.get("bytes_in") or 0) + (s.get("bytes_out") or 0),
            })

        # 5. Check for tokens (privilege traces)
        tokens = db.all_tokens()
        active_tokens = [t for t in tokens if t.get("active")]
        for t in active_tokens:
            artifacts.append({
                "type": "active_token",
                "token_id": t.get("id"),
                "agent_id": t.get("agent_id"),
                "username": t.get("username"),
                "domain": t.get("domain"),
                "source": t.get("source"),
            })

        # Generate recommendations
        if artifacts:
            recommendations.append("Review and clear audit logs for sensitive operations")
        if active_socks:
            recommendations.append(f"Close {len(active_socks)} active SOCKS session(s)")
        if active_tokens:
            recommendations.append(f"Revoke {len(active_tokens)} active token(s)")
        sensitive_count = sum(1 for a in artifacts if a["type"] == "sensitive_command")
        if sensitive_count:
            recommendations.append(f"Review {sensitive_count} task(s) with sensitive commands")

        type_counts = {}
        for a in artifacts:
            t = a["type"]
            type_counts[t] = type_counts.get(t, 0) + 1

        output_lines = [f"Found {len(artifacts)} artifacts to review"]
        for t, c in sorted(type_counts.items(), key=lambda x: -x[1]):
            output_lines.append(f"  {t}: {c}")
        if recommendations:
            output_lines.append("")
            output_lines.append("Recommendations:")
            for r in recommendations:
                output_lines.append(f"  - {r}")

        write_result(
            True,
            output="\n".join(output_lines),
            data={
                "total_artifacts": len(artifacts),
                "type_counts": type_counts,
                "artifacts": artifacts[:200],
                "recommendations": recommendations,
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
