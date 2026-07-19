#!/usr/bin/env python3
"""Credential Report plugin — generates a detailed credential collection summary."""

import json
import os
import sys
from datetime import datetime

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_report

HIGH_VALUE_USERS = {"administrator", "admin", "root", "sa", "krbtgt", "system"}


def generate_html(creds: dict, title: str, redacted: bool) -> str:
    all_creds = creds["all"]
    type_counts = creds["type_counts"]
    source_counts = creds["source_counts"]
    domain_counts = creds["domain_counts"]
    high_value = creds["high_value"]

    html = f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>{title}</title>
<style>
body {{ font-family: -apple-system, sans-serif; max-width: 900px; margin: 0 auto; padding: 20px; color: #1a1a2e; }}
h1 {{ color: #0f3460; border-bottom: 3px solid #e94560; padding-bottom: 10px; }}
h2 {{ color: #16213e; margin-top: 25px; }}
table {{ width: 100%; border-collapse: collapse; margin: 10px 0; }}
th, td {{ border: 1px solid #ddd; padding: 6px 10px; text-align: left; font-size: 12px; }}
th {{ background: #0f3460; color: white; }}
tr:nth-child(even) {{ background: #f8f9fa; }}
.hv {{ background: #fff3cd !important; font-weight: bold; }}
.stat {{ display: inline-block; background: #e8f4f8; border-left: 4px solid #0f3460; padding: 10px 15px; margin: 5px; border-radius: 4px; }}
.stat .num {{ font-size: 24px; font-weight: bold; color: #e94560; }}
.stat .label {{ font-size: 12px; color: #666; }}
</style></head><body>
<h1>{title}</h1>
<p>Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')} | Total: {len(all_creds)} | Redacted: {'Yes' if redacted else 'No'}</p>

<h2>Summary</h2>
<div>
  <div class="stat"><div class="num">{len(all_creds)}</div><div class="label">Total Credentials</div></div>
  <div class="stat"><div class="num">{len(high_value)}</div><div class="label">High-Value</div></div>
  <div class="stat"><div class="num">{sum(1 for c in all_creds if c.get('confirmed'))}</div><div class="label">Confirmed</div></div>
</div>

<h2>By Type</h2>
<table><tr><th>Type</th><th>Count</th></tr>"""
    for k, v in sorted(type_counts.items(), key=lambda x: -x[1]):
        html += f"<tr><td>{k}</td><td>{v}</td></tr>"
    html += "</table>"

    html += """<h2>By Source</h2>
<table><tr><th>Source</th><th>Count</th></tr>"""
    for k, v in sorted(source_counts.items(), key=lambda x: -x[1]):
        html += f"<tr><td>{k}</td><td>{v}</td></tr>"
    html += "</table>"

    if domain_counts:
        html += """<h2>By Domain</h2>
<table><tr><th>Domain</th><th>Count</th></tr>"""
        for k, v in sorted(domain_counts.items(), key=lambda x: -x[1]):
            html += f"<tr><td>{k}</td><td>{v}</td></tr>"
        html += "</table>"

    if high_value:
        html += """<h2>High-Value Accounts</h2>
<table><tr><th>Username</th><th>Domain</th><th>Type</th><th>Source</th><th>Confirmed</th></tr>"""
        for c in high_value:
            html += f"""<tr class="hv">
<td>{c['username']}</td><td>{c['domain']}</td><td>{c['type']}</td>
<td>{c['source']}</td><td>{'Yes' if c['confirmed'] else 'No'}</td></tr>"""
        html += "</table>"

    html += "</body></html>"
    return html


def main():
    data_input = read_stdin()
    params = data_input.get("params", {})
    config = data_input.get("config", {})

    fmt = params.get("format", "html")
    title = params.get("title", "Credential Collection Report")
    redacted = config.get("redact_passwords", "true").lower() == "true"

    try:
        db = Database()
    except FileNotFoundError as e:
        print(json.dumps({"title": title, "format": fmt, "content": f"Error: {e}"}))
        return

    try:
        all_creds = db.all_credentials()

        type_counts = {}
        source_counts = {}
        domain_counts = {}
        high_value = []

        for c in all_creds:
            t = c.get("type") or "unknown"
            type_counts[t] = type_counts.get(t, 0) + 1
            s = c.get("source") or "unknown"
            source_counts[s] = source_counts.get(s, 0) + 1
            d = c.get("domain") or "N/A"
            domain_counts[d] = domain_counts.get(d, 0) + 1

            username = (c.get("username") or "").lower()
            if any(hv in username for hv in HIGH_VALUE_USERS):
                entry = {
                    "username": c.get("username"),
                    "domain": c.get("domain"),
                    "type": c.get("type"),
                    "source": c.get("source"),
                    "confirmed": c.get("confirmed"),
                }
                if redacted:
                    entry["password"] = "***REDACTED***" if c.get("password") else ""
                    entry["hash"] = "***REDACTED***" if c.get("hash") else ""
                else:
                    entry["password"] = c.get("password", "")
                    entry["hash"] = c.get("hash", "")
                high_value.append(entry)

        creds_data = {
            "all": all_creds,
            "type_counts": type_counts,
            "source_counts": source_counts,
            "domain_counts": domain_counts,
            "high_value": high_value,
        }

        if fmt == "json":
            output = {
                "total": len(all_creds),
                "type_counts": type_counts,
                "source_counts": source_counts,
                "domain_counts": domain_counts,
                "high_value": high_value,
            }
            content = json.dumps(output, indent=2, default=str)
        elif fmt == "markdown":
            lines = [
                f"# {title}", f"*Total: {len(all_creds)}*", "",
                "## By Type", "| Type | Count |", "|---|---|",
            ]
            for k, v in sorted(type_counts.items(), key=lambda x: -x[1]):
                lines.append(f"| {k} | {v} |")
            if high_value:
                lines += ["", "## High-Value Accounts", "| User | Domain | Type |", "|---|---|---|"]
                for c in high_value:
                    lines.append(f"| {c['username']} | {c['domain']} | {c['type']} |")
            content = "\n".join(lines)
        else:
            content = generate_html(creds_data, title, redacted)
            fmt = "html"

        write_report(title, content, fmt)
    finally:
        db.close()


if __name__ == "__main__":
    main()
