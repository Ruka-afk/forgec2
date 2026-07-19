#!/usr/bin/env python3
"""Browser Data Recon plugin — analyzes browser history, bookmarks, cookies, and saved credentials."""

import json
import os
import re
import sqlite3
import sys
import tempfile
from collections import Counter, defaultdict
from urllib.parse import urlparse

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

# ── Domain classification ───────────────────────────────────────────

DOMAIN_CATEGORIES = {
    "financial": [
        "bank", "paypal", "venmo", "chase", "wellsfargo", "wells Fargo",
        "citi", "boa", "americanexpress", "amex", "capitalone", "fidelity",
        "vanguard", "schwab", "etrade", "robinhood", "coinbase", "binance",
        "stripe", "square", "zelle", "wise", "revolut", "goldmansachs",
        "investing", "stock", "trading", "finance", "payment",
    ],
    "email": [
        "mail.google.com", "outlook", "hotmail", "protonmail", "proton.me",
        "yahoo.com/mail", "icloud.com/mail", "zoho.com/mail",
        "fastmail", "tutanota", "gmx.com", "aol.com/mail",
    ],
    "cloud": [
        "console.cloud.google.com", "console.aws.amazon.com",
        "portal.azure.com", "console.azure", "console.amazon",
        "s3.amazonaws", "storage.googleapis", "cloudflare",
        "digitalocean", "heroku", "netlify", "vercel", "linode",
        "vultr", "ovh", "backblaze", "dropbox", "drive.google",
        "onedrive", "box.com", "icloud.com", "mega.nz",
    ],
    "social": [
        "facebook.com", "twitter.com", "x.com", "instagram.com",
        "linkedin.com", "reddit.com", "tiktok.com", "snapchat.com",
        "pinterest.com", "tumblr.com", "mastodon", "threads.net",
        "discord.com", "telegram.org", "whatsapp.com", "signal.org",
        "wechat.com", "line.me", "viber.com",
    ],
    "admin": [
        "admin", "login", "auth", "sso", "okta", "onelogin",
        "jumpcloud", "ping", "keycloak", "ldap", "active directory",
        "sharepoint", "confluence", "jira", "gitlab", "jenkins",
        "grafana", "kibana", "nagios", "zabbix", "datadog",
    ],
    "development": [
        "github.com", "gitlab.com", "bitbucket.org", "stackoverflow.com",
        "npmjs.com", "pypi.org", "hub.docker.com", "docker.com",
        "rubygems.org", "crates.io", "nuget.org", "maven",
        "npm", "yarn", "pip", "cargo", "gem",
    ],
}

SENSITIVE_BOOKMARK_KEYWORDS = [
    "bank", "admin", "vpn", "cloud", "github", "aws", "azure",
    "console", "portal", "login", "auth", "sso", "keycloak",
    "jenkins", "gitlab", "grafana", "kibana", "database", "db",
    "firewall", "router", "switch", "nas", "backup", "server",
    "internal", "corp", "intranet", "password", "vault", "secret",
    "ssh", "rdp", "remote", "vpn", "proxy", "tunnel",
]

HIGH_VALUE_DOMAINS = [
    "bank", "paypal", "chase", "wellsfargo", "citi", "boa",
    "americanexpress", "capitalone", "fidelity", "vanguard",
    "coinbase", "binance", "mail.google.com", "outlook",
    "protonmail", "proton.me", "console.cloud.google.com",
    "console.aws.amazon.com", "portal.azure.com", "github.com",
    "gitlab.com", "dropbox.com", "drive.google.com",
]

DOWNLOAD_EXTENSIONS = {
    "executable": [".exe", ".msi", ".bat", ".cmd", ".ps1", ".com", ".scr", ".pif"],
    "archive": [".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".tar.gz"],
    "script": [".js", ".vbs", ".wsf", ".hta", ".py", ".rb", ".pl"],
    "document": [".doc", ".docx", ".pdf", ".xls", ".xlsx", ".ppt", ".pptx"],
}

CHROME_HISTORY_QUERY = (
    "SELECT url, title, visit_count, last_visit_time FROM urls ORDER BY visit_count DESC"
)
CHROME_COOKIES_QUERY = (
    "SELECT host_key, name, path, expires_utc, is_secure, is_httponly "
    "FROM cookies WHERE host_key != ''"
)
CHROME_BOOKMARKS_QUERY = (
    "SELECT name, url FROM bookmarks WHERE url IS NOT NULL AND url != ''"
)
FIREFOX_HISTORY_QUERY = (
    "SELECT url, title, visit_count FROM moz_places WHERE visit_count > 0 "
    "ORDER BY visit_count DESC"
)


def _classify_domain(domain: str) -> str:
    domain_lower = domain.lower()
    for category, keywords in DOMAIN_CATEGORIES.items():
        for kw in keywords:
            if kw in domain_lower:
                return category
    return "other"


def _is_high_value_domain(domain: str) -> bool:
    domain_lower = domain.lower()
    for hv in HIGH_VALUE_DOMAINS:
        if hv in domain_lower:
            return True
    return False


def _is_sensitive_bookmark(name: str, url: str) -> bool:
    combined = f"{name} {url}".lower()
    for kw in SENSITIVE_BOOKMARK_KEYWORDS:
        if kw in combined:
            return True
    return False


def _extract_domain(url: str) -> str:
    try:
        parsed = urlparse(url)
        host = parsed.hostname or ""
        parts = host.split(".")
        if len(parts) >= 2:
            return ".".join(parts[-2:])
        return host
    except Exception:
        return ""


def _detect_browser_type(db_path: str) -> str:
    basename = os.path.basename(db_path).lower()
    if basename in ("history", "places.sqlite"):
        parent = os.path.basename(os.path.dirname(db_path)).lower()
        if "firefox" in parent or "mozilla" in parent:
            return "firefox"
        return "chrome"
    return "unknown"


def _detect_browser_name(db_path: str) -> str:
    path_lower = db_path.lower().replace("\\", "/")
    if "firefox" in path_lower or "mozilla" in path_lower:
        return "Firefox"
    if "edge" in path_lower:
        return "Microsoft Edge"
    if "brave" in path_lower:
        return "Brave"
    if "opera" in path_lower:
        return "Opera"
    if "vivaldi" in path_lower:
        return "Vivaldi"
    if "chromium" in path_lower:
        return "Chromium"
    return "Chrome"


def _detect_profile(db_path: str) -> str:
    path_lower = db_path.lower().replace("\\", "/")
    for profile in ["default", "profile 1", "profile 2", "profile 3",
                     "guest profile", "private", "incognito"]:
        if profile in path_lower:
            return profile.title()
    return "Default"


def _detect_browser_version(db_path: str) -> str:
    path_lower = db_path.lower().replace("\\", "/")
    match = re.search(r"chrome[/\\\\](\d+\.\d+\.\d+\.\d+)", path_lower)
    if match:
        return match.group(1)
    return "unknown"


def _query_sqlite_db(db_path: str, query: str):
    try:
        conn = sqlite3.connect(f"file:{db_path}?mode=ro", uri=True)
        conn.row_factory = sqlite3.Row
        rows = conn.execute(query).fetchall()
        conn.close()
        return [dict(r) for r in rows]
    except Exception:
        return []


def _process_chrome_history(db_path: str, top_n: int):
    rows = _query_sqlite_db(db_path, CHROME_HISTORY_QUERY)
    domain_counter = Counter()
    for r in rows:
        domain = _extract_domain(r.get("url", ""))
        if domain:
            domain_counter[domain] += r.get("visit_count", 1)
    top_sites = [
        {"domain": d, "visits": c, "category": _classify_domain(d)}
        for d, c in domain_counter.most_common(top_n)
    ]
    return rows, top_sites


def _process_chrome_cookies(db_path: str):
    rows = _query_sqlite_db(db_path, CHROME_COOKIES_QUERY)
    cookies = []
    high_value = []
    for r in rows:
        host = r.get("host_key", "")
        domain = _extract_domain(host) if host.startswith(".") else host
        entry = {
            "domain": host,
            "name": r.get("name", ""),
            "secure": bool(r.get("is_secure")),
            "httponly": bool(r.get("is_httponly")),
        }
        cookies.append(entry)
        if _is_high_value_domain(host):
            high_value.append(entry)
    return cookies, high_value


def _process_chrome_bookmarks(db_path: str):
    rows = _query_sqlite_db(db_path, CHROME_BOOKMARKS_QUERY)
    bookmarks = []
    sensitive = []
    for r in rows:
        name = r.get("name", "")
        url = r.get("url", "")
        entry = {"name": name, "url": url, "domain": _extract_domain(url)}
        bookmarks.append(entry)
        if _is_sensitive_bookmark(name, url):
            sensitive.append(entry)
    return bookmarks, sensitive


def _process_firefox_history(db_path: str, top_n: int):
    rows = _query_sqlite_db(db_path, FIREFOX_HISTORY_QUERY)
    domain_counter = Counter()
    for r in rows:
        domain = _extract_domain(r.get("url", ""))
        if domain:
            domain_counter[domain] += r.get("visit_count", 1)
    top_sites = [
        {"domain": d, "visits": c, "category": _classify_domain(d)}
        for d, c in domain_counter.most_common(top_n)
    ]
    return rows, top_sites


def _parse_chrome_downloads(db_path: str):
    query = (
        "SELECT target_path, total_bytes, start_time, end_time "
        "FROM downloads ORDER BY start_time DESC"
    )
    rows = _query_sqlite_db(db_path, query)
    downloads = []
    for r in rows:
        target = r.get("target_path", "")
        ext = os.path.splitext(target)[1].lower() if target else ""
        category = "other"
        for cat, exts in DOWNLOAD_EXTENSIONS.items():
            if ext in exts:
                category = cat
                break
        downloads.append({
            "path": target,
            "size_bytes": r.get("total_bytes", 0),
            "extension": ext,
            "category": category,
        })
    return downloads


def _parse_chrome_login_data(db_path: str):
    query = "SELECT origin_url, username_value FROM logins WHERE username_value != ''"
    return _query_sqlite_db(db_path, query)


def _process_browser(db_path: str, top_n: int):
    browser_type = _detect_browser_type(db_path)
    browser_name = _detect_browser_name(db_path)
    browser_version = _detect_browser_version(db_path)
    profile_name = _detect_profile(db_path)

    profile_data = {
        "name": profile_name,
        "history_count": 0,
        "top_sites": [],
        "saved_creds": [],
        "cookies": [],
        "high_value_cookies": [],
        "bookmarks": [],
        "sensitive_bookmarks": [],
        "downloads": [],
    }

    if browser_type == "chrome":
        history, top_sites = _process_chrome_history(db_path, top_n)
        profile_data["history_count"] = len(history)
        profile_data["top_sites"] = top_sites
        cookies, high_value = _process_chrome_cookies(db_path)
        profile_data["cookies"] = cookies[:top_n]
        profile_data["high_value_cookies"] = high_value
        bookmarks, sensitive = _process_chrome_bookmarks(db_path)
        profile_data["bookmarks"] = bookmarks[:top_n]
        profile_data["sensitive_bookmarks"] = sensitive
        profile_data["downloads"] = _parse_chrome_downloads(db_path)[:top_n]
        profile_data["saved_creds"] = [
            {"domain": r.get("origin_url", ""), "username": r.get("username_value", "")}
            for r in _parse_chrome_login_data(db_path)
        ]
    elif browser_type == "firefox":
        history, top_sites = _process_firefox_history(db_path, top_n)
        profile_data["history_count"] = len(history)
        profile_data["top_sites"] = top_sites

    return {
        "name": browser_name,
        "version": browser_version,
        "type": browser_type,
        "db_path": db_path,
        "profiles": [profile_data],
    }


def _scan_task_results(db, agent_id=None):
    """Scan task results for browser-related data."""
    if agent_id:
        tasks = db.tasks_for_agent(agent_id)
    else:
        tasks = db.all_tasks()

    browser_db_paths = set()
    for task in tasks:
        result = task.get("result") or ""
        if not result:
            continue
        try:
            result_data = json.loads(result) if isinstance(result, str) else result
        except (json.JSONDecodeError, TypeError):
            result_data = {}

        output = ""
        if isinstance(result_data, dict):
            output = result_data.get("output", "")
        if isinstance(output, str):
            browser_db_paths.update(_find_browser_paths_in_text(output))

    return browser_db_paths


def _find_browser_paths_in_text(text):
    patterns = [
        r"[\\/](?:Chrome|Edge|Brave|Opera|Vivaldi|Chromium)[\\/][^\"'\s]*[\\/]History",
        r"[\\/](?:Chrome|Edge|Brave|Opera|Vivaldi|Chromium)[\\/][^\"'\s]*[\\/]Login Data",
        r"[\\/](?:Chrome|Edge|Brave|Opera|Vivaldi|Chromium)[\\/][^\"'\s]*[\\/]Cookies",
        r"[\\/](?:Chrome|Edge|Brave|Opera|Vivaldi|Chromium)[\\/][^\"'\s]*[\\/]Bookmarks",
        r"[\\/](?:Firefox|Mozilla)[\\/][^\"'\s]*[\\/]places\.sqlite",
        r"[\\/](?:Firefox|Mozilla)[\\/][^\"'\s]*[\\/]logins\.json",
        r"[\\/](?:Firefox|Mozilla)[\\/][^\"'\s]*[\\/]cookies\.sqlite",
        r"sqlite3? .*(?:History|Login Data|Cookies|Bookmarks|places\.sqlite)",
    ]
    found = set()
    for pat in patterns:
        for match in re.finditer(pat, text, re.IGNORECASE):
            found.add(match.group(0).strip())
    return found


def _scan_task_results_for_browser_data(db, agent_id=None):
    """Parse task results looking for browser SQLite query output or file contents."""
    if agent_id:
        tasks = db.tasks_for_agent(agent_id)
    else:
        tasks = db.all_tasks()

    browsers = {}

    for task in tasks:
        result = task.get("result") or ""
        if not result:
            continue
        try:
            result_data = json.loads(result) if isinstance(result, str) else result
        except (json.JSONDecodeError, TypeError):
            continue

        if not isinstance(result_data, dict):
            continue

        output = result_data.get("output", "")
        data = result_data.get("data", {})
        if not isinstance(output, str):
            continue

        output_lower = output.lower()

        browser_name = None
        browser_type = None
        if "firefox" in output_lower or "mozilla" in output_lower:
            browser_name = "Firefox"
            browser_type = "firefox"
        elif "edge" in output_lower:
            browser_name = "Microsoft Edge"
            browser_type = "chrome"
        elif "chrome" in output_lower or "chromium" in output_lower:
            browser_name = "Chrome"
            browser_type = "chrome"

        if browser_name is None:
            continue

        if browser_name not in browsers:
            browsers[browser_name] = {
                "name": browser_name,
                "version": "unknown",
                "type": browser_type,
                "profiles": [{
                    "name": "Default",
                    "history_count": 0,
                    "top_sites": [],
                    "saved_creds": [],
                    "cookies": [],
                    "high_value_cookies": [],
                    "bookmarks": [],
                    "sensitive_bookmarks": [],
                    "downloads": [],
                }],
            }

        profile = browsers[browser_name]["profiles"][0]

        if isinstance(data, dict):
            if "urls" in data or "history" in data:
                entries = data.get("urls") or data.get("history") or []
                if isinstance(entries, list):
                    profile["history_count"] += len(entries)
                    domain_counter = Counter()
                    for e in entries:
                        url = e.get("url", "") if isinstance(e, dict) else str(e)
                        domain = _extract_domain(url)
                        if domain:
                            domain_counter[domain] += e.get("visit_count", 1) if isinstance(e, dict) else 1
                    for d, c in domain_counter.most_common(20):
                        profile["top_sites"].append({
                            "domain": d, "visits": c, "category": _classify_domain(d),
                        })

            if "cookies" in data:
                entries = data["cookies"]
                if isinstance(entries, list):
                    for e in entries:
                        if isinstance(e, dict):
                            host = e.get("host_key") or e.get("host") or e.get("domain", "")
                            entry = {
                                "domain": host,
                                "name": e.get("name", ""),
                                "secure": e.get("secure", False),
                                "httponly": e.get("httponly", False),
                            }
                            profile["cookies"].append(entry)
                            if _is_high_value_domain(host):
                                profile["high_value_cookies"].append(entry)

            if "bookmarks" in data:
                entries = data["bookmarks"]
                if isinstance(entries, list):
                    for e in entries:
                        if isinstance(e, dict):
                            name = e.get("name", "")
                            url = e.get("url", "")
                            entry = {"name": name, "url": url, "domain": _extract_domain(url)}
                            profile["bookmarks"].append(entry)
                            if _is_sensitive_bookmark(name, url):
                                profile["sensitive_bookmarks"].append(entry)

            if "logins" in data or "saved_passwords" in data:
                entries = data.get("logins") or data.get("saved_passwords") or []
                if isinstance(entries, list):
                    for e in entries:
                        if isinstance(e, dict):
                            profile["saved_creds"].append({
                                "domain": e.get("origin_url") or e.get("domain", ""),
                                "username": e.get("username") or e.get("username_value", ""),
                            })

            if "downloads" in data:
                entries = data["downloads"]
                if isinstance(entries, list):
                    for e in entries:
                        if isinstance(e, dict):
                            target = e.get("target_path") or e.get("path", "")
                            ext = os.path.splitext(target)[1].lower()
                            category = "other"
                            for cat, exts in DOWNLOAD_EXTENSIONS.items():
                                if ext in exts:
                                    category = cat
                                    break
                            profile["downloads"].append({
                                "path": target,
                                "size_bytes": e.get("total_bytes", 0),
                                "extension": ext,
                                "category": category,
                            })

        profile["history_count"] = profile.get("history_count", 0) or len(profile.get("top_sites", []))

    return list(browsers.values())


def main():
    data = read_stdin()
    params = data.get("params", {})
    config = data.get("config", {})
    agent_id = params.get("agent_id") or config.get("agent_id")
    top_n = int(params.get("top_n") or config.get("top_n", 20))

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        browsers = _scan_task_results_for_browser_data(db, agent_id)

        total_history = sum(
            p.get("history_count", 0)
            for b in browsers
            for p in b.get("profiles", [])
        )
        total_saved_creds = sum(
            len(p.get("saved_creds", []))
            for b in browsers
            for p in b.get("profiles", [])
        )
        high_value_cookies = sum(
            len(p.get("high_value_cookies", []))
            for b in browsers
            for p in b.get("profiles", [])
        )
        sensitive_bookmarks = sum(
            len(p.get("sensitive_bookmarks", []))
            for b in browsers
            for p in b.get("profiles", [])
        )

        total_browsers = len(browsers)

        output_lines = [
            f"Browsers found: {total_browsers}",
            f"Total history entries: {total_history}",
            f"Saved credentials: {total_saved_creds}",
            f"High-value cookies: {high_value_cookies}",
            f"Sensitive bookmarks: {sensitive_bookmarks}",
        ]

        summary = {
            "total_browsers": total_browsers,
            "total_history": total_history,
            "total_saved_creds": total_saved_creds,
            "high_value_cookies": high_value_cookies,
            "sensitive_bookmarks": sensitive_bookmarks,
        }

        write_result(
            True,
            output="\n".join(output_lines),
            data={
                "browsers": browsers,
                "summary": summary,
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
