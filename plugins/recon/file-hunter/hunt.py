#!/usr/bin/env python3
import sys, os, re
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

# ---------------------------------------------------------------------------
# Patterns
# ---------------------------------------------------------------------------

PRIVATE_KEY_PATTERNS = re.compile(
    r"(?:^|[/\\])(?:"
    r"[\w.-]*\.pem"
    r"|[\w.-]*\.key"
    r"|[\w.-]*\.p12"
    r"|[\w.-]*\.pfx"
    r"|[\w.-]*\.jks"
    r"|id_rsa(?:\.pub)?"
    r"|id_ed25519(?:\.pub)?"
    r"|id_ecdsa(?:\.pub)?"
    r"|id_dsa(?:\.pub)?"
    r")$", re.IGNORECASE,
)

CERTIFICATE_PATTERNS = re.compile(
    r"(?:^|[/\\])(?:"
    r"[\w.-]*\.crt"
    r"|[\w.-]*\.cer"
    r"|[\w.-]*\.ca-bundle"
    r")$", re.IGNORECASE,
)

CONFIG_PATTERNS = re.compile(
    r"(?:^|[/\\])(?:"
    r"[\w.-]*\.conf"
    r"|[\w.-]*\.cfg"
    r"|[\w.-]*\.ini"
    r"|[\w.-]*\.env"
    r"|\.env[\w.-]*"
    r"|web\.config"
    r"|appsettings\.json"
    r"|docker-compose\.ya?ml"
    r")$", re.IGNORECASE,
)

DATABASE_PATTERNS = re.compile(
    r"(?:^|[/\\])(?:"
    r"[\w.-]*\.db"
    r"|[\w.-]*\.sqlite"
    r"|[\w.-]*\.sqlite3"
    r"|[\w.-]*\.mdb"
    r")$", re.IGNORECASE,
)

DOCUMENT_PATTERNS = re.compile(
    r"(?:^|[/\\])(?:"
    r"[\w.-]*\.docx"
    r"|[\w.-]*\.xlsx"
    r"|[\w.-]*\.pdf"
    r"|[\w.-]*\.pptx"
    r")$", re.IGNORECASE,
)

PASSWORD_STORE_PATTERNS = re.compile(
    r"(?:^|[/\\])(?:"
    r"[\w.-]*\.kdbx"
    r"|[\w.-]*\.keychain"
    r"|[\w.-]*\.keystore"
    r")$", re.IGNORECASE,
)

CREDENTIAL_LINE_RE = re.compile(
    r"(?:password|secret|api_key|token|passwd|credentials?)\s*[=:]\s*\S+",
    re.IGNORECASE,
)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

CMD_LS_RE = re.compile(
    r"^\s*\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}.*?\s+[\d,]+\s+(.+)$",
)

FILE_LINE_RE = re.compile(
    r"(?:^|[\s|])([/\\]?[/\\]?[\w./\\-]+\.\w{1,10})(?:\s|$|,|\|)",
)


def _risk_for(path: str) -> str:
    name = os.path.basename(path).lower()
    if PRIVATE_KEY_PATTERNS.search(path) or name in {
        "id_rsa", "id_ed25519", "id_ecdsa", "id_dsa",
    }:
        return "CRITICAL"
    if PASSWORD_STORE_PATTERNS.search(path):
        return "CRITICAL"
    if name.startswith(".env") or name.endswith(".env"):
        return "CRITICAL"
    if CERTIFICATE_PATTERNS.search(path):
        return "HIGH"
    if DATABASE_PATTERNS.search(path):
        return "HIGH"
    if CONFIG_PATTERNS.search(path):
        return "HIGH"
    if DOCUMENT_PATTERNS.search(path):
        return "MEDIUM"
    return "MEDIUM"


def _file_type_for(path: str) -> str:
    if PRIVATE_KEY_PATTERNS.search(path):
        return "keys"
    if PASSWORD_STORE_PATTERNS.search(path):
        return "keys"
    if CERTIFICATE_PATTERNS.search(path):
        return "certificates"
    if DATABASE_PATTERNS.search(path):
        return "databases"
    if CONFIG_PATTERNS.search(path):
        return "configs"
    if DOCUMENT_PATTERNS.search(path):
        return "documents"
    return "other"


def _parse_file_lines(output: str) -> list[str]:
    """Extract file paths from ls/dir/find/tree-like output."""
    paths: list[str] = []
    for line in output.splitlines():
        for m in FILE_LINE_RE.finditer(line):
            p = m.group(1).strip().strip('"').strip("'")
            if p and not p.startswith(("-", "total")):
                paths.append(p)
    return paths


def _scan_for_credentials(output: str) -> list[str]:
    """Return lines that look like inline credentials."""
    hits: list[str] = []
    for line in output.splitlines():
        if CREDENTIAL_LINE_RE.search(line):
            hits.append(line.strip())
    return hits

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


def run():
    db = Database()
    params = read_stdin()

    agent_filter = params.get("agent_id")
    max_results = int(params.get("max_results", 100))

    agents = db.get_agents()
    if agent_filter:
        agents = [a for a in agents if a.get("id") == agent_filter]

    result_agents = []
    all_files: list[dict] = []
    by_risk = {"critical": 0, "high": 0, "medium": 0}
    by_type: dict[str, int] = {}

    for agent in agents:
        agent_id = agent.get("id", "")
        hostname = agent.get("hostname", "unknown")
        tasks = db.get_tasks(agent_id)

        seen: set[str] = set()
        agent_files: list[dict] = []

        for task in tasks:
            output = task.get("output", "")
            if not output:
                continue

            paths = _parse_file_lines(output)

            creds = _scan_for_credentials(output)
            if creds:
                for c in creds:
                    tag = "CREDENTIAL_IN_FILE"
                    if tag not in seen:
                        seen.add(tag)
                        risk = "CRITICAL"
                        ftype = "credentials"
                        entry = {
                            "path": "[inline] " + c[:120],
                            "type": ftype,
                            "risk": risk,
                            "size": 0,
                        }
                        agent_files.append(entry)
                        by_risk["critical"] += 1
                        by_type["credentials"] = by_type.get("credentials", 0) + 1

            for p in paths:
                if p in seen:
                    continue
                seen.add(p)

                risk = _risk_for(p)
                ftype = _file_type_for(p)
                entry = {"path": p, "type": ftype, "risk": risk, "size": 0}
                agent_files.append(entry)
                by_risk[risk.lower()] += 1
                by_type[ftype] = by_type.get(ftype, 0) + 1

                if len(agent_files) >= max_results:
                    break
            if len(agent_files) >= max_results:
                break

        if agent_files:
            result_agents.append({
                "id": agent_id,
                "hostname": hostname,
                "files": agent_files,
            })
            all_files.extend(agent_files)

    total_agents = len(result_agents)
    total_files = len(all_files)

    summary = {
        "total_agents": total_agents,
        "total_files": total_files,
        "by_risk": by_risk,
        "by_type": by_type,
    }

    human = (
        f"Scanned {total_agents} agents | "
        f"{total_files} sensitive files | "
        f"{by_risk['critical']} critical | "
        f"{by_risk['high']} high"
    )

    data = {"agents": result_agents, "summary": summary}
    write_result(True, output=human, data=data)


if __name__ == "__main__":
    run()
