#!/usr/bin/env python3
"""MITRE ATT&CK Mapper plugin — maps task results to ATT&CK techniques and tactics."""

import json
import os
import sys
from collections import defaultdict
from datetime import datetime

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_report

# ── Technique mapping ─────────────────────────────────────────
TASK_TECHNIQUE_MAP = {
    "shell":        ("T1059", "Command and Scripting Interpreter", "Execution"),
    "powershell":   ("T1059.001", "PowerShell", "Execution"),
    "screenshot":   ("T1113", "Screen Capture", "Collection"),
    "keylog_start": ("T1056.001", "Keylogging", "Collection"),
    "browser_creds":("T1555", "Credentials from Password Stores", "Credential Access"),
    "kerberoast":   ("T1558.003", "Kerberoasting", "Credential Access"),
    "hashdump":     ("T1003.002", "Security Account Manager", "Credential Access"),
    "upload":       ("T1041", "Exfiltration Over C2 Channel", "Exfiltration"),
    "download":     ("T1041", "Exfiltration Over C2 Channel", "Exfiltration"),
    "portscan":     ("T1046", "Network Service Discovery", "Discovery"),
    "beacon_config":("T1572", "Protocol Tunneling", "Command and Control"),
    "dl":           ("T1005", "Data from Local System", "Collection"),
    "cat":          ("T1005", "Data from Local System", "Collection"),
    "rm":           ("T1485", "Data Destruction", "Impact"),
    "kill":         ("T1485", "Data Destruction", "Impact"),
}

# Patterns checked with startswith
TASK_PREFIX_MAP = [
    ("persist_",    "T1053", "Scheduled Task/Job", "Persistence"),
    ("lateral_",    "T1021", "Remote Services", "Lateral Movement"),
]

TACTICS_ORDER = [
    ("TA0001", "Initial Access"),
    ("TA0002", "Execution"),
    ("TA0003", "Persistence"),
    ("TA0004", "Privilege Escalation"),
    ("TA0005", "Defense Evasion"),
    ("TA0006", "Credential Access"),
    ("TA0007", "Discovery"),
    ("TA0008", "Lateral Movement"),
    ("TA0009", "Collection"),
    ("TA0010", "Exfiltration"),
    ("TA0040", "Impact"),
]

TACTIC_RECOMMENDATIONS = {
    "Initial Access":    ["T1566 (Phishing)", "T1190 (Exploit Public-Facing App)", "T1078 (Valid Accounts)"],
    "Execution":         ["T1059 (Command Interpreter)", "T1203 (Exploitation for Client Execution)", "T1047 (WMI)"],
    "Persistence":       ["T1547 (Boot/Logon Autostart)", "T1053 (Scheduled Task)", "T1136 (Create Account)"],
    "Privilege Escalation": ["T1068 (Exploitation for Priv Esc)", "T1055 (Process Injection)", "T1134 (Access Token)"],
    "Defense Evasion":   ["T1027 (Obfuscated Files)", "T1070 (Indicator Removal)", "T1562 (Impair Defenses)"],
    "Credential Access": ["T1003 (OS Credential Dumping)", "T1110 (Brute Force)", "T1558 (Steal/Forge Kerberos)"],
    "Discovery":         ["T1087 (Account Discovery)", "T1082 (System Info Discovery)", "T1049 (System Network)"],
    "Lateral Movement":  ["T1021 (Remote Services)", "T1570 (Lateral Tool Transfer)"],
    "Collection":        ["T1005 (Data from Local)", "T1039 (Data from Network)", "T1113 (Screen Capture)"],
    "Exfiltration":      ["T1041 (Over C2)", "T1048 (Over Alternative Protocol)", "T1567 (Over Web Service)"],
    "Impact":            ["T1485 (Data Destruction)", "T1486 (Data Encrypted)", "T1499 (Endpoint DoS)"],
}

DEFAULT_TECHNIQUE = ("T1059", "Command and Scripting Interpreter", "Execution")


def classify_task(task_type: str):
    if task_type in TASK_TECHNIQUE_MAP:
        return TASK_TECHNIQUE_MAP[task_type]
    for prefix, tid, tname, tactic in TASK_PREFIX_MAP:
        if task_type.startswith(prefix):
            return (tid, tname, tactic)
    return DEFAULT_TECHNIQUE


def aggregate_tasks(tasks):
    technique_counts = defaultdict(int)
    tactic_counts = defaultdict(int)
    task_type_counts = defaultdict(int)

    for t in tasks:
        ttype = t.get("type", "") or t.get("command", "").split()[0] if t.get("command") else ""
        if not ttype:
            continue
        tid, tname, tactic = classify_task(ttype)
        key = f"{tid}: {tname}"
        technique_counts[key] += 1
        tactic_counts[tactic] += 1
        task_type_counts[ttype] += 1

    return dict(technique_counts), dict(tactic_counts), dict(task_type_counts)


def generate_html(title, tasks, techniques, tactics, task_types):
    total_tasks = len(tasks)
    uncovered = [tname for _, tname in TACTICS_ORDER if tname not in tactics]
    tactic_lookup = {tname: tid for tid, tname in TACTICS_ORDER}

    bar_width = 40
    heatmap_lines = []
    for tid, tname in TACTICS_ORDER:
        count = tactics.get(tname, 0)
        pct = (count / total_tasks * 100) if total_tasks else 0
        filled = int(pct / 100 * bar_width)
        bar = "\u2588" * filled + "\u2591" * (bar_width - filled)
        heatmap_lines.append((tname, tid, count, pct, bar))

    html = f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>{title}</title>
<style>
body {{ font-family: -apple-system, sans-serif; max-width: 1000px; margin: 0 auto; padding: 20px; color: #1a1a2e; }}
h1 {{ color: #0f3460; border-bottom: 3px solid #e94560; padding-bottom: 10px; }}
h2 {{ color: #16213e; margin-top: 30px; }}
table {{ width: 100%; border-collapse: collapse; margin: 15px 0; }}
th, td {{ border: 1px solid #ddd; padding: 8px 12px; text-align: left; font-size: 13px; }}
th {{ background: #0f3460; color: white; }}
tr:nth-child(even) {{ background: #f8f9fa; }}
.bar {{ font-family: monospace; letter-spacing: -1px; color: #e94560; }}
.covered {{ color: #28a745; font-weight: bold; }}
.uncovered {{ color: #dc3545; font-weight: bold; }}
.pct {{ font-size: 11px; color: #666; }}
.rec {{ font-size: 12px; color: #444; margin-left: 10px; }}
</style></head><body>
<h1>{title}</h1>
<p>Generated: {datetime.now().strftime("%Y-%m-%d %H:%M:%S")} | Total Tasks: {total_tasks} | Techniques Observed: {len(techniques)} | Tactics Covered: {len(tactics)}/{len(TACTICS_ORDER)}</p>

<h2>Tactic Heatmap</h2>
<table><tr><th>Tactic</th><th>ID</th><th>Count</th><th>Coverage</th><th style="min-width:300px">Heatmap</th></tr>"""
    for tname, tid, count, pct, bar in heatmap_lines:
        cls = "covered" if count > 0 else "uncovered"
        html += f'<tr><td class="{cls}">{tname}</td><td>{tid}</td><td>{count}</td><td class="pct">{pct:.1f}%</td><td class="bar">{bar}</td></tr>'
    html += "</table>"

    html += """<h2>Techniques Observed</h2>
<table><tr><th>Technique</th><th>Count</th><th>% of Tasks</th></tr>"""
    for tkey, count in sorted(techniques.items(), key=lambda x: -x[1]):
        pct = count / total_tasks * 100 if total_tasks else 0
        html += f"<tr><td>{tkey}</td><td>{count}</td><td>{pct:.1f}%</td></tr>"
    html += "</table>"

    if uncovered:
        html += "<h2>Recommendations — Uncovered Tactics</h2><ul>"
        for tname in uncovered:
            tid = tactic_lookup.get(tname, "?")
            recs = TACTIC_RECOMMENDATIONS.get(tname, [])
            html += f'<li><span class="uncovered">{tname} ({tid})</span><ul class="rec">'
            for r in recs:
                html += f"<li>{r}</li>"
            html += "</ul></li>"
        html += "</ul>"

    html += f"""<h2>Task Type Breakdown</h2>
<table><tr><th>Task Type</th><th>Count</th></tr>"""
    for ttype, count in sorted(task_types.items(), key=lambda x: -x[1]):
        html += f"<tr><td>{ttype}</td><td>{count}</td></tr>"
    html += "</table></body></html>"
    return html


def generate_markdown(title, tasks, techniques, tactics, task_types):
    total_tasks = len(tasks)
    lines = [
        f"# {title}",
        f"*Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')} | Tasks: {total_tasks} | Techniques: {len(techniques)} | Tactics: {len(tactics)}/{len(TACTICS_ORDER)}*",
        "",
        "## Tactic Heatmap",
        "| Tactic | ID | Count | Coverage |",
        "|---|---|---|---|",
    ]
    for tid, tname in TACTICS_ORDER:
        count = tactics.get(tname, 0)
        pct = (count / total_tasks * 100) if total_tasks else 0
        lines.append(f"| {tname} | {tid} | {count} | {pct:.1f}% |")

    lines += ["", "## Techniques Observed", "| Technique | Count |", "|---|---|"]
    for tkey, count in sorted(techniques.items(), key=lambda x: -x[1]):
        lines.append(f"| {tkey} | {count} |")

    uncovered = [tname for _, tname in TACTICS_ORDER if tname not in tactics]
    if uncovered:
        lines += ["", "## Recommendations — Uncovered Tactics"]
        for tname in uncovered:
            lines.append(f"- **{tname}**")
            for r in TACTIC_RECOMMENDATIONS.get(tname, []):
                lines.append(f"  - {r}")

    lines += ["", "## Task Type Breakdown", "| Type | Count |", "|---|---|"]
    for ttype, count in sorted(task_types.items(), key=lambda x: -x[1]):
        lines.append(f"| {ttype} | {count} |")
    return "\n".join(lines)


def generate_json(title, tasks, techniques, tactics, task_types):
    return json.dumps({
        "title": title,
        "generated": datetime.now().isoformat(),
        "total_tasks": len(tasks),
        "techniques": techniques,
        "tactics": tactics,
        "task_types": task_types,
    }, indent=2)


def main():
    data_input = read_stdin()
    params = data_input.get("params", {})

    fmt = params.get("format", "html")
    title = params.get("title", "MITRE ATT&CK Mapping")

    try:
        db = Database()
    except FileNotFoundError as e:
        print(json.dumps({"title": title, "format": fmt, "content": f"Error: {e}"}))
        return

    try:
        tasks = db.all_tasks()
        techniques, tactics, task_types = aggregate_tasks(tasks)

        if fmt == "json":
            content = generate_json(title, tasks, techniques, tactics, task_types)
        elif fmt == "markdown":
            content = generate_markdown(title, tasks, techniques, tactics, task_types)
        else:
            content = generate_html(title, tasks, techniques, tactics, task_types)
            fmt = "html"

        write_report(title, content, fmt)
    finally:
        db.close()


if __name__ == "__main__":
    main()
