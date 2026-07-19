#!/usr/bin/env python3
import hashlib, os, re, sys
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

# ---------------------------------------------------------------------------
# Regex patterns
# ---------------------------------------------------------------------------

JWT_RE = re.compile(r"eyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]+")

API_KEY_PREFIX_RE = re.compile(
    r"(?:^|[\s\"'=,;:({\[})"
    r"(sk-[a-zA-Z0-9_-]{20,})"
    r"|(ak_[a-zA-Z0-9_-]{20,})"
    r"|(AKIA[A-Z0-9]{16})"
    r"|(ghp_[a-zA-Z0-9_-]{36,})"
    r"|(glpat-[a-zA-Z0-9_-]{20,})",
)

API_KEY_GENERIC_RE = re.compile(
    r"api[_-]?key\s*[=:]\s*['\"]?([a-zA-Z0-9_-]{20,})['\"]?",
    re.IGNORECASE,
)

SESSION_COOKIE_RE = re.compile(
    r"(?:^|;\s*)"
    r"(session|token|auth|jwt|PHPSESSID|JSESSIONID|connect\.sid)"
    r"\s*=\s*([^\s;]{8,})",
    re.IGNORECASE,
)

OAUTH_RE = re.compile(
    r"(access_token|refresh_token)\s*=\s*([^\s\"'&;]{20,})",
    re.IGNORECASE,
)

AWS_KEY_RE = re.compile(r"AKIA[A-Z0-9]{16}")
YANDEX_KEY_RE = re.compile(r"ya[0-9]{40}")
GCP_KEY_RE = re.compile(r"AIza[A-Za-z0-9_-]{35}")

SSH_KEY_RE = re.compile(
    r"-----BEGIN (RSA |OPENSSH |EC |DSA )?PRIVATE KEY-----"
)

X509_CERT_RE = re.compile(
    r"-----BEGIN CERTIFICATE-----"
)

BROWSER_COOKIES_RE = re.compile(
    r"(?:Cookies|cookies)\s+(?:file|path)\s*[:=]\s*([^\s\"]+)",
    re.IGNORECASE,
)

LOCAL_STATE_RE = re.compile(
    r"(?:Local\s*State|local_state)\s*(?:file|path)?\s*[:=]\s*([^\s\"]+)",
    re.IGNORECASE,
)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_TYPE_ORDER = [
    "jwt", "api_key", "aws_key", "gcp_key", "yandex_key",
    "session_cookie", "oauth_token", "ssh_key", "x509_cert",
    "browser_token",
]


def _hash(value: str) -> str:
    return hashlib.sha256(value.encode()).hexdigest()[:16]


def _preview(value: str, length: int = 40) -> str:
    if len(value) <= length:
        return value
    return value[: length - 3] + "..."


def _extract_jwt(text: str) -> list[dict]:
    hits = []
    for m in JWT_RE.finditer(text):
        token = m.group(0)
        hits.append({"type": "jwt", "value": token})
    return hits


def _extract_api_keys(text: str) -> list[dict]:
    hits = []
    for m in API_KEY_PREFIX_RE.finditer(text):
        val = next((g for g in m.groups() if g), None)
        if val:
            hits.append({"type": "api_key", "value": val})
    for m in API_KEY_GENERIC_RE.finditer(text):
        hits.append({"type": "api_key", "value": m.group(1)})
    return hits


def _extract_cloud_keys(text: str) -> list[dict]:
    hits = []
    for m in AWS_KEY_RE.finditer(text):
        hits.append({"type": "aws_key", "value": m.group(0)})
    for m in GCP_KEY_RE.finditer(text):
        hits.append({"type": "gcp_key", "value": m.group(0)})
    for m in YANDEX_KEY_RE.finditer(text):
        hits.append({"type": "yandex_key", "value": m.group(0)})
    return hits


def _extract_cookies(text: str) -> list[dict]:
    hits = []
    for m in SESSION_COOKIE_RE.finditer(text):
        hits.append({
            "type": "session_cookie",
            "value": f"{m.group(1)}={m.group(2)}",
        })
    return hits


def _extract_oauth(text: str) -> list[dict]:
    hits = []
    for m in OAuth_RE.finditer(text):
        hits.append({
            "type": "oauth_token",
            "value": f"{m.group(1)}={m.group(2)}",
        })
    return hits


def _extract_ssh_keys(text: str) -> list[dict]:
    hits = []
    for m in SSH_KEY_RE.finditer(text):
        hits.append({"type": "ssh_key", "value": m.group(0)[:80]})
    return hits


def _extract_certs(text: str) -> list[dict]:
    hits = []
    for m in X509_CERT_RE.finditer(text):
        hits.append({"type": "x509_cert", "value": m.group(0)[:80]})
    return hits


def _extract_browser_tokens(text: str) -> list[dict]:
    hits = []
    for m in BROWSER_COOKIES_RE.finditer(text):
        hits.append({"type": "browser_token", "value": f"cookie_file={m.group(1)}"})
    for m in LOCAL_STATE_RE.finditer(text):
        hits.append({"type": "browser_token", "value": f"local_state={m.group(1)}"})
    return hits


def _scan_text(text: str) -> list[dict]:
    """Run all extractors against a block of text."""
    all_hits = []
    all_hits.extend(_extract_jwt(text))
    all_hits.extend(_extract_api_keys(text))
    all_hits.extend(_extract_cloud_keys(text))
    all_hits.extend(_extract_cookies(text))
    all_hits.extend(_extract_oauth(text))
    all_hits.extend(_extract_ssh_keys(text))
    all_hits.extend(_extract_certs(text))
    all_hits.extend(_extract_browser_tokens(text))
    return all_hits


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def run():
    data = read_stdin()
    params = data.get("params", {})
    agent_filter = params.get("agent_id", "")

    try:
        db = Database()
    except FileNotFoundError as exc:
        write_result(False, error=str(exc))
        return

    try:
        agents = db.all_agents()
        if agent_filter:
            agents = [
                a for a in agents
                if a["id"] == agent_filter or a["id"][:8] == agent_filter
            ]

        seen_hashes: set[str] = set()
        tokens: list[dict] = []
        by_type: dict[str, int] = {t: 0 for t in _TYPE_ORDER}
        scanned_agents = 0

        for agent in agents:
            agent_id = agent["id"]
            hostname = agent.get("hostname", "")
            tasks = db.tasks_for_agent(agent_id)
            scanned_agents += 1

            for task in tasks:
                result_text = task.get("result", "") or task.get("output", "") or ""
                if not result_text:
                    continue

                hits = _scan_text(result_text)
                for hit in hits:
                    h = _hash(hit["value"])
                    if h in seen_hashes:
                        continue
                    seen_hashes.add(h)

                    hit_type = hit["type"]
                    by_type[hit_type] = by_type.get(hit_type, 0) + 1
                    tokens.append({
                        "type": hit_type,
                        "value_preview": _preview(hit["value"]),
                        "source": hostname or agent_id[:8],
                        "agent_id": agent_id,
                    })

        total = len(tokens)
        jwt_count = by_type.get("jwt", 0)
        api_count = sum(by_type.get(k, 0) for k in ("api_key", "aws_key", "gcp_key", "yandex_key"))
        cert_count = by_type.get("x509_cert", 0)

        summary = {
            "total": total,
            "by_type": {k: v for k, v in by_type.items() if v},
        }

        human = (
            f"Scanned {scanned_agents} agents | "
            f"{total} tokens | "
            f"{jwt_count} JWT | "
            f"{api_count} API keys | "
            f"{cert_count} certificates"
        )

        write_result(True, output=human, data={"tokens": tokens, "summary": summary})
    finally:
        db.close()


if __name__ == "__main__":
    run()
