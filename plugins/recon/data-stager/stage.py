#!/usr/bin/env python3
"""Data Stager plugin — analyzes collected data and stages information for exfiltration."""

import json
import os
import sys
from pathlib import Path

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

HIGH_VALUE_EXTENSIONS = {
    ".docx", ".xlsx", ".pdf", ".key", ".pem", ".pfx",
    ".kdbx", ".sqlite", ".db", ".p12", ".crt",
}


def _scan_uploads_dir(agent_id: str) -> list:
    """Check data/uploads/<agent_id>/ for collected files."""
    uploads = []
    candidates = ["data/uploads", "../data/uploads"]
    for base in candidates:
        agent_dir = Path(base) / agent_id
        if agent_dir.is_dir():
            for f in agent_dir.rglob("*"):
                if f.is_file():
                    ext = f.suffix.lower()
                    uploads.append({
                        "name": f.name,
                        "path": str(f),
                        "size_bytes": f.stat().st_size,
                        "high_value": ext in HIGH_VALUE_EXTENSIONS,
                    })
            break
    return uploads


def _analyze_tasks(tasks: list) -> dict:
    """Analyze download/upload tasks for a single agent."""
    total_files = 0
    total_size = 0
    high_value = []
    staged = []

    for t in tasks:
        ttype = t.get("type", "")
        if ttype not in ("download", "download_url", "upload"):
            continue

        path = t.get("path") or t.get("command") or ""
        filename = Path(path).name if path else ""
        ext = Path(filename).suffix.lower() if filename else ""
        size = t.get("total_bytes") or t.get("size") or 0
        transferred = t.get("transferred") or 0
        status = t.get("status", "")

        if filename:
            total_files += 1
        total_size += size

        entry = {
            "task_id": t.get("id"),
            "filename": filename,
            "path": path,
            "size_bytes": size,
            "transferred": transferred,
            "status": status,
            "high_value": ext in HIGH_VALUE_EXTENSIONS,
        }
        staged.append(entry)
        if ext in HIGH_VALUE_EXTENSIONS:
            high_value.append(entry)

    return {
        "total_files": total_files,
        "total_size_bytes": total_size,
        "high_value_files": high_value,
        "staged_data": staged,
    }


def main():
    data = read_stdin()
    agent_id = data.get("agent_id", "")
    config = data.get("config", {})
    filter_path = config.get("path", "")

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        agents = db.all_agents()
        if agent_id:
            agents = [a for a in agents if a.get("id") == agent_id]

        agent_results = []
        total_data_bytes = 0
        high_value_count = 0
        by_agent = {}

        for a in agents:
            aid = a.get("id", "")
            hostname = a.get("hostname") or aid[:8]

            tasks = db.tasks_for_agent(aid)
            analysis = _analyze_tasks(tasks)

            uploads = _scan_uploads_dir(aid)
            for u in uploads:
                analysis["staged_data"].append(u)
                analysis["total_files"] += 1
                analysis["total_size_bytes"] += u["size_bytes"]
                if u["high_value"]:
                    analysis["high_value_files"].append(u)

            total_data_bytes += analysis["total_size_bytes"]
            high_value_count += len(analysis["high_value_files"])

            agent_entry = {
                "id": aid,
                "hostname": hostname,
                "os": a.get("os"),
                "status": a.get("status"),
                "total_files": analysis["total_files"],
                "total_size_bytes": analysis["total_size_bytes"],
                "high_value_files": analysis["high_value_files"],
                "staged_data": analysis["staged_data"],
            }
            agent_results.append(agent_entry)
            by_agent[hostname] = {
                "total_files": analysis["total_files"],
                "total_size_bytes": analysis["total_size_bytes"],
                "high_value_count": len(analysis["high_value_files"]),
            }

        recommendations = []
        if high_value_count > 0:
            recommendations.append(
                f"Prioritize exfiltration of {high_value_count} high-value file(s)"
            )
        if total_data_bytes > 100 * 1024 * 1024:
            recommendations.append(
                "Large data volume detected — consider chunked exfiltration"
            )
        for entry in agent_results:
            if entry["total_files"] > 20:
                recommendations.append(
                    f"{entry['hostname']}: {entry['total_files']} files ready — "
                    "batch download recommended"
                )

        summary = {
            "total_agents": len(agent_results),
            "total_data_bytes": total_data_bytes,
            "high_value_count": high_value_count,
            "by_agent": by_agent,
        }

        if total_data_bytes >= 1024 * 1024:
            size_str = f"{total_data_bytes / (1024 * 1024):.1f} MB"
        elif total_data_bytes >= 1024:
            size_str = f"{total_data_bytes / 1024:.1f} KB"
        else:
            size_str = f"{total_data_bytes} B"

        output = (
            f"Analyzed {len(agent_results)} agent(s) | "
            f"{size_str} total data | "
            f"{high_value_count} high-value file(s) identified"
        )

        write_result(
            True,
            output=output,
            data={
                "agents": agent_results,
                "summary": summary,
                "recommendations": recommendations,
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
