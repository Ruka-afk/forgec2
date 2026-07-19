"""Shared database helper for ForgeC2 plugins.

Discovers the SQLite database automatically and provides query helpers.
All plugins import this module: ``from lib.db import Database``.
"""

import json
import os
import sqlite3
import sys
from pathlib import Path
from typing import Any, Dict, List, Optional


def _find_db() -> Optional[str]:
    """Walk up from the plugin directory to locate the SQLite database."""
    # data/forgec2.db is the default; also check forgec2.db directly
    candidates = ["data/forgec2.db", "forgec2.db"]
    # Start from the working directory (set by executor to plugin dir)
    d = Path.cwd()
    for _ in range(6):
        for c in candidates:
            p = d / c
            if p.is_file():
                return str(p)
        d = d.parent
    # Fallback: environment variable
    env = os.environ.get("FORGEC2_DB_PATH")
    if env and os.path.isfile(env):
        return env
    return None


class Database:
    """Lightweight SQLite reader for plugin scripts."""

    def __init__(self, path: Optional[str] = None):
        if path is None:
            path = _find_db()
        if path is None:
            raise FileNotFoundError("Cannot locate ForgeC2 database")
        self.path = path
        self._conn = sqlite3.connect(f"file:{path}?mode=ro", uri=True)
        self._conn.row_factory = sqlite3.Row

    def close(self):
        self._conn.close()

    # ── Agents ──────────────────────────────────────────────
    def all_agents(self) -> List[Dict[str, Any]]:
        rows = self._conn.execute(
            "SELECT * FROM implants ORDER BY last_seen DESC"
        ).fetchall()
        return [dict(r) for r in rows]

    def agent_by_id(self, agent_id: str) -> Optional[Dict[str, Any]]:
        r = self._conn.execute(
            "SELECT * FROM implants WHERE id = ?", (agent_id,)
        ).fetchone()
        return dict(r) if r else None

    def agent_count(self) -> int:
        return self._conn.execute("SELECT COUNT(*) FROM implants").fetchone()[0]

    def agents_by_status(self, status: str) -> List[Dict[str, Any]]:
        rows = self._conn.execute(
            "SELECT * FROM implants WHERE status = ?", (status,)
        ).fetchall()
        return [dict(r) for r in rows]

    def agents_by_os(self) -> Dict[str, int]:
        rows = self._conn.execute(
            "SELECT os, COUNT(*) as cnt FROM implants GROUP BY os"
        ).fetchall()
        return {r["os"]: r["cnt"] for r in rows}

    # ── Tasks ───────────────────────────────────────────────
    def all_tasks(self) -> List[Dict[str, Any]]:
        rows = self._conn.execute(
            "SELECT * FROM tasks ORDER BY created_at DESC"
        ).fetchall()
        return [dict(r) for r in rows]

    def tasks_for_agent(self, agent_id: str) -> List[Dict[str, Any]]:
        rows = self._conn.execute(
            "SELECT * FROM tasks WHERE agent_id = ? ORDER BY created_at DESC",
            (agent_id,),
        ).fetchall()
        return [dict(r) for r in rows]

    def task_count(self) -> int:
        return self._conn.execute("SELECT COUNT(*) FROM tasks").fetchone()[0]

    def tasks_by_status(self) -> Dict[str, int]:
        rows = self._conn.execute(
            "SELECT status, COUNT(*) as cnt FROM tasks GROUP BY status"
        ).fetchall()
        return {r["status"]: r["cnt"] for r in rows}

    def tasks_by_type(self) -> Dict[str, int]:
        rows = self._conn.execute(
            "SELECT type, COUNT(*) as cnt FROM tasks GROUP BY type ORDER BY cnt DESC"
        ).fetchall()
        return {r["type"]: r["cnt"] for r in rows}

    # ── Listeners ───────────────────────────────────────────
    def all_listeners(self) -> List[Dict[str, Any]]:
        rows = self._conn.execute(
            "SELECT * FROM listeners ORDER BY created_at DESC"
        ).fetchall()
        return [dict(r) for r in rows]

    # ── Credentials ─────────────────────────────────────────
    def all_credentials(self) -> List[Dict[str, Any]]:
        rows = self._conn.execute(
            "SELECT * FROM credential_entries ORDER BY created_at DESC"
        ).fetchall()
        return [dict(r) for r in rows]

    def credential_count(self) -> int:
        return self._conn.execute(
            "SELECT COUNT(*) FROM credential_entries"
        ).fetchone()[0]

    def credentials_by_type(self) -> Dict[str, int]:
        rows = self._conn.execute(
            "SELECT type, COUNT(*) as cnt FROM credential_entries GROUP BY type"
        ).fetchall()
        return {r["type"]: r["cnt"] for r in rows}

    def credentials_by_source(self) -> Dict[str, int]:
        rows = self._conn.execute(
            "SELECT source, COUNT(*) as cnt FROM credential_entries GROUP BY source"
        ).fetchall()
        return {r["source"]: r["cnt"] for r in rows}

    # ── Audit Logs ──────────────────────────────────────────
    def all_audit_logs(self) -> List[Dict[str, Any]]:
        rows = self._conn.execute(
            "SELECT * FROM audit_logs ORDER BY created_at DESC"
        ).fetchall()
        return [dict(r) for r in rows]

    def audit_logs_since(self, since: str) -> List[Dict[str, Any]]:
        rows = self._conn.execute(
            "SELECT * FROM audit_logs WHERE created_at >= ? ORDER BY created_at",
            (since,),
        ).fetchall()
        return [dict(r) for r in rows]

    def audit_actions(self) -> Dict[str, int]:
        rows = self._conn.execute(
            "SELECT action, COUNT(*) as cnt FROM audit_logs GROUP BY action ORDER BY cnt DESC"
        ).fetchall()
        return {r["action"]: r["cnt"] for r in rows}

    # ── Scan Results ────────────────────────────────────────
    def all_scan_results(self) -> List[Dict[str, Any]]:
        rows = self._conn.execute(
            "SELECT * FROM scan_results ORDER BY created_at DESC"
        ).fetchall()
        return [dict(r) for r in rows]

    # ── Network Hosts ───────────────────────────────────────
    def all_network_hosts(self) -> List[Dict[str, Any]]:
        rows = self._conn.execute(
            "SELECT * FROM network_hosts ORDER BY last_seen DESC"
        ).fetchall()
        return [dict(r) for r in rows]

    # ── Build Logs ──────────────────────────────────────────
    def all_build_logs(self) -> List[Dict[str, Any]]:
        rows = self._conn.execute(
            "SELECT * FROM build_logs ORDER BY created_at DESC"
        ).fetchall()
        return [dict(r) for r in rows]

    # ── Tokens ──────────────────────────────────────────────
    def all_tokens(self) -> List[Dict[str, Any]]:
        rows = self._conn.execute(
            "SELECT * FROM token_entries ORDER BY created_at DESC"
        ).fetchall()
        return [dict(r) for r in rows]

    # ── SOCKS Sessions ──────────────────────────────────────
    def all_socks_sessions(self) -> List[Dict[str, Any]]:
        rows = self._conn.execute(
            "SELECT * FROM socks_sessions ORDER BY created_at DESC"
        ).fetchall()
        return [dict(r) for r in rows]

    # ── Users ───────────────────────────────────────────────
    def all_users(self) -> List[Dict[str, Any]]:
        rows = self._conn.execute(
            "SELECT id, username, role, is_active, last_login, last_ip, created_at FROM users"
        ).fetchall()
        return [dict(r) for r in rows]

    def user_count(self) -> int:
        return self._conn.execute("SELECT COUNT(*) FROM users").fetchone()[0]

    # ── Plugins ─────────────────────────────────────────────
    def all_plugins(self) -> List[Dict[str, Any]]:
        rows = self._conn.execute(
            "SELECT * FROM plugins ORDER BY name"
        ).fetchall()
        return [dict(r) for r in rows]


def read_stdin() -> Dict[str, Any]:
    """Read and parse JSON from stdin (the ForgeC2 executor protocol)."""
    raw = sys.stdin.read()
    if not raw:
        return {}
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        return {}


def write_result(
    success: bool,
    output: str = "",
    data: Optional[Dict[str, Any]] = None,
    error: str = "",
) -> None:
    """Write a Result JSON to stdout (the ForgeC2 executor protocol)."""
    result: Dict[str, Any] = {"success": success, "output": output}
    if data is not None:
        result["data"] = data
    if error:
        result["error"] = error
    print(json.dumps(result))


def write_report(title: str, content: str, fmt: str = "html") -> None:
    """Write a Report JSON to stdout (the ForgeC2 executor protocol)."""
    report = {"title": title, "format": fmt, "content": content}
    print(json.dumps(report))
