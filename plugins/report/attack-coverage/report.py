#!/usr/bin/env python3
"""ATT&CK Coverage Dashboard — visual heatmap, technique coverage, gap analysis."""

import json
import os
import sys
from collections import defaultdict
from datetime import datetime

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_report

# ── Task type → ATT&CK technique mapping ──────────────────────
TASK_TECHNIQUE_MAP = {
    "shell":         ("T1059",    "Command and Scripting Interpreter", "Execution"),
    "powershell":    ("T1059.001","PowerShell",                        "Execution"),
    "screenshot":    ("T1113",    "Screen Capture",                   "Collection"),
    "keylog_start":  ("T1056.001","Keylogging",                       "Collection"),
    "browser_creds": ("T1555",    "Credentials from Password Stores", "Credential Access"),
    "kerberoast":    ("T1558.003","Kerberoasting",                    "Credential Access"),
    "hashdump":      ("T1003.002","Security Account Manager",         "Credential Access"),
    "upload":        ("T1041",    "Exfiltration Over C2 Channel",     "Exfiltration"),
    "download":      ("T1041",    "Exfiltration Over C2 Channel",     "Exfiltration"),
    "portscan":      ("T1046",    "Network Service Discovery",        "Discovery"),
    "beacon_config": ("T1572",    "Protocol Tunneling",               "Command and Control"),
    "dl":            ("T1005",    "Data from Local System",           "Collection"),
    "cat":           ("T1005",    "Data from Local System",           "Collection"),
    "rm":            ("T1485",    "Data Destruction",                 "Impact"),
    "kill":          ("T1485",    "Data Destruction",                 "Impact"),
    "ls":            ("T1083",    "File and Directory Discovery",     "Discovery"),
    "ps":            ("T1057",    "Process Discovery",                "Discovery"),
    "whoami":        ("T1033",    "System Owner/User Discovery",      "Discovery"),
    "ipconfig":      ("T1049",    "System Network Connections Disc.",  "Discovery"),
    "tasklist":      ("T1057",    "Process Discovery",                "Discovery"),
    "net_user":      ("T1087.001","Local Account Discovery",          "Discovery"),
    "net_group":     ("T1069.001","Local Groups Discovery",           "Discovery"),
    "mimikatz":      ("T1003.001","LSASS Memory",                     "Credential Access"),
    "seatbelt":      ("T1518.001","Security Software Discovery",      "Discovery"),
    "sharphound":    ("T1087.002","Domain Account Discovery",         "Discovery"),
    "rubeus":        ("T1558",    "Steal or Forge Kerberos Tickets",  "Credential Access"),
    "portfwd":       ("T1090",    "Proxy",                            "Command and Control"),
    "webcam":        ("T1125",    "Video Capture",                    "Collection"),
    "mic":           ("T1123",    "Audio Capture",                    "Collection"),
    "keylog_stop":   ("T1056.001","Keylogging",                       "Collection"),
    "checkin":       ("T1572",    "Protocol Tunneling",               "Command and Control"),
    "execute":       ("T1204.002","User Execution: Malicious File",   "Execution"),
    "inject":        ("T1055",    "Process Injection",                "Defense Evasion"),
    "uac_bypass":    ("T1548.002","Bypass User Account Control",      "Defense Evasion"),
    "amsi_bypass":   ("T1562.001","Impair Defenses: Disable or Modify Tools", "Defense Evasion"),
    "etw_bypass":    ("T1562.006","Impair Defenses: Indicator Blocking",      "Defense Evasion"),
}

TASK_PREFIX_MAP = [
    ("persist_",  "T1053", "Scheduled Task/Job",            "Persistence"),
    ("lateral_",  "T1021", "Remote Services",               "Lateral Movement"),
    ("registry_", "T1547.001","Registry Run Keys / Startup Folder", "Persistence"),
    ("service_",  "T1543.003","Windows Service",            "Persistence"),
    ("startup_",  "T1547.001","Registry Run Keys / Startup Folder", "Persistence"),
    ("cron_",     "T1053.003","Cron Job",                    "Persistence"),
]

# One entry per tactic
TACTICS_DEDUPED = [
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
    ("TA0011", "Command and Control"),
    ("TA0040", "Impact"),
    ("TA0042", "Resource Development"),
    ("TA0043", "Reconnaissance"),
]

TACTIC_RECOMMENDATIONS = {
    "Initial Access":        ["T1566 (Phishing)", "T1190 (Exploit Public-Facing App)", "T1078 (Valid Accounts)"],
    "Execution":             ["T1059 (Command Interpreter)", "T1203 (Exploitation for Client Exec)", "T1047 (WMI)"],
    "Persistence":           ["T1547 (Boot/Logon Autostart)", "T1053 (Scheduled Task)", "T1136 (Create Account)"],
    "Privilege Escalation":  ["T1068 (Exploitation for Priv Esc)", "T1055 (Process Injection)", "T1134 (Access Token)"],
    "Defense Evasion":       ["T1027 (Obfuscated Files)", "T1070 (Indicator Removal)", "T1562 (Impair Defenses)"],
    "Credential Access":     ["T1003 (OS Credential Dumping)", "T1110 (Brute Force)", "T1558 (Steal/Forge Kerberos)"],
    "Discovery":             ["T1087 (Account Discovery)", "T1082 (System Info Discovery)", "T1049 (System Network)"],
    "Lateral Movement":      ["T1021 (Remote Services)", "T1570 (Lateral Tool Transfer)"],
    "Collection":            ["T1005 (Data from Local)", "T1039 (Data from Network)", "T1113 (Screen Capture)"],
    "Exfiltration":          ["T1041 (Over C2)", "T1048 (Over Alt Protocol)", "T1567 (Over Web Service)"],
    "Command and Control":   ["T1572 (Protocol Tunneling)", "T1071 (Application Layer Protocol)", "T1573 (Encrypted Channel)"],
    "Impact":                ["T1485 (Data Destruction)", "T1486 (Data Encrypted)", "T1499 (Endpoint DoS)"],
    "Resource Development":  ["T1583 (Acquire Infrastructure)", "T1587 (Develop Capabilities)", "T1585 (Establish Accounts)"],
    "Reconnaissance":        ["T1595 (Active Scanning)", "T1592 (Gather Victim Host Info)", "T1589 (Gather Victim Identity)"],
}

# Reference: known ATT&CK techniques per tactic for full coverage analysis
KNOWN_TECHNIQUES = {
    "Initial Access": [
        ("T1189", "Drive-by Compromise"),
        ("T1190", "Exploit Public-Facing Application"),
        ("T1133", "External Remote Services"),
        ("T1200", "Hardware Additions"),
        ("T1566", "Phishing"),
        ("T1078", "Valid Accounts"),
    ],
    "Execution": [
        ("T1059", "Command and Scripting Interpreter"),
        ("T1203", "Exploitation for Client Execution"),
        ("T1559", "Inter-Process Communication"),
        ("T1106", "Native API"),
        ("T1053", "Scheduled Task/Job"),
        ("T1129", "Shared Modules"),
        ("T1072", "Software Deployment Tools"),
        ("T1569", "System Services"),
        ("T1204", "User Execution"),
        ("T1047", "Windows Management Instrumentation"),
    ],
    "Persistence": [
        ("T1098", "Account Manipulation"),
        ("T1197", "BITS Jobs"),
        ("T1547", "Boot or Logon Autostart Execution"),
        ("T1037", "Boot or Logon Initialization Scripts"),
        ("T1176", "Browser Extensions"),
        ("T1554", "Compromise Client Software Binary"),
        ("T1136", "Create Account"),
        ("T1543", "Create or Modify System Process"),
        ("T1546", "Event Triggered Execution"),
        ("T1574", "Hijack Execution Flow"),
        ("T1525", "Implant Internal Image"),
        ("T1556", "Modify Authentication Process"),
        ("T1137", "Office Application Startup"),
        ("T1542", "Pre-OS Boot"),
        ("T1505", "Server Software Component"),
        ("T1053", "Scheduled Task/Job"),
    ],
    "Privilege Escalation": [
        ("T1548", "Abuse Elevation Control Mechanism"),
        ("T1134", "Access Token Manipulation"),
        ("T1068", "Exploitation for Privilege Escalation"),
        ("T1055", "Process Injection"),
    ],
    "Defense Evasion": [
        ("T1140", "Deobfuscate/Decode Files"),
        ("T1006", "Direct Volume Access"),
        ("T1484", "Domain Policy Modification"),
        ("T1480", "Execution Guardrails"),
        ("T1211", "Exploitation for Defense Evasion"),
        ("T1222", "File and Directory Permissions Modification"),
        ("T1564", "Hide Artifacts"),
        ("T1562", "Impair Defenses"),
        ("T1070", "Indicator Removal"),
        ("T1202", "Indirect Command Execution"),
        ("T1036", "Masquerading"),
        ("T1556", "Modify Authentication Process"),
        ("T1578", "Modify Cloud Compute Infrastructure"),
        ("T1112", "Modify Registry"),
        ("T1027", "Obfuscated Files or Information"),
        ("T1542", "Pre-OS Boot"),
        ("T1218", "System Binary Proxy Execution"),
        ("T1216", "System Script Proxy Execution"),
        ("T1221", "Template Injection"),
        ("T1205", "Traffic Signaling"),
        ("T1127", "Trusted Developer Utilities Proxy Execution"),
        ("T1535", "Unused/Unsupported Cloud Regions"),
        ("T1550", "Use Alternate Authentication Material"),
        ("T1497", "Virtualization/Sandbox Evasion"),
    ],
    "Credential Access": [
        ("T1557", "Adversary-in-the-Middle"),
        ("T1110", "Brute Force"),
        ("T1555", "Credentials from Password Stores"),
        ("T1212", "Exploitation for Credential Access"),
        ("T1187", "Forced Authentication"),
        ("T1606", "Forge Web Credentials"),
        ("T1056", "Input Capture"),
        ("T1556", "Modify Authentication Process"),
        ("T1111", "Multi-Factor Authentication Interception"),
        ("T1003", "OS Credential Dumping"),
        ("T1528", "Steal Application Access Token"),
        ("T1649", "Steal or Forge Authentication Certificates"),
        ("T1558", "Steal or Forge Kerberos Tickets"),
        ("T1539", "Steal Web Session Cookie"),
        ("T1552", "Unsecured Credentials"),
    ],
    "Discovery": [
        ("T1087", "Account Discovery"),
        ("T1010", "Application Window Discovery"),
        ("T1217", "Browser Bookmark Discovery"),
        ("T1580", "Cloud Infrastructure Discovery"),
        ("T1538", "Cloud Service Dashboard"),
        ("T1526", "Cloud Service Discovery"),
        ("T1619", "Cloud Storage Object Discovery"),
        ("T1622", "Debugger Evasion"),
        ("T1482", "Domain Trust Discovery"),
        ("T1083", "File and Directory Discovery"),
        ("T1615", "Group Policy Discovery"),
        ("T1046", "Network Service Discovery"),
        ("T1135", "Network Share Discovery"),
        ("T1040", "Network Sniffing"),
        ("T1201", "Password Policy Discovery"),
        ("T1120", "Peripheral Device Discovery"),
        ("T1069", "Permission Groups Discovery"),
        ("T1057", "Process Discovery"),
        ("T1012", "Query Registry"),
        ("T1018", "Remote System Discovery"),
        ("T1518", "Software Discovery"),
        ("T1082", "System Information Discovery"),
        ("T1614", "System Location Discovery"),
        ("T1016", "System Network Configuration Discovery"),
        ("T1049", "System Network Connections Discovery"),
        ("T1033", "System Owner/User Discovery"),
        ("T1007", "System Service Discovery"),
        ("T1124", "System Time Discovery"),
    ],
    "Lateral Movement": [
        ("T1210", "Exploitation of Remote Services"),
        ("T1534", "Internal Spearphishing"),
        ("T1570", "Lateral Tool Transfer"),
        ("T1563", "Remote Service Session Hijacking"),
        ("T1021", "Remote Services"),
        ("T1080", "Taint Shared Content"),
        ("T1072", "Software Deployment Tools"),
        ("T1550", "Use Alternate Authentication Material"),
    ],
    "Collection": [
        ("T1557", "Adversary-in-the-Middle"),
        ("T1560", "Archive Collected Data"),
        ("T1123", "Audio Capture"),
        ("T1119", "Automated Collection"),
        ("T1185", "Browser Session Hijacking"),
        ("T1115", "Clipboard Data"),
        ("T1530", "Data from Cloud Storage"),
        ("T1602", "Data from Configuration Repository"),
        ("T1213", "Data from Information Repositories"),
        ("T1005", "Data from Local System"),
        ("T1039", "Data from Network Shared Drive"),
        ("T1025", "Data from Removable Media"),
        ("T1074", "Data Staged"),
        ("T1114", "Email Collection"),
        ("T1056", "Input Capture"),
        ("T1113", "Screen Capture"),
        ("T1125", "Video Capture"),
    ],
    "Exfiltration": [
        ("T1020", "Automated Exfiltration"),
        ("T1030", "Data Transfer Size Limits"),
        ("T1048", "Exfiltration Over Alternative Protocol"),
        ("T1041", "Exfiltration Over C2 Channel"),
        ("T1011", "Exfiltration Over Other Network Medium"),
        ("T1052", "Exfiltration Over Physical Medium"),
        ("T1567", "Exfiltration Over Web Service"),
        ("T1029", "Scheduled Transfer"),
        ("T1537", "Transfer Data to Cloud Account"),
    ],
    "Command and Control": [
        ("T1071", "Application Layer Protocol"),
        ("T1092", "Communication Through Removable Media"),
        ("T1132", "Data Encoding"),
        ("T1001", "Data Obfuscation"),
        ("T1568", "Dynamic Resolution"),
        ("T1573", "Encrypted Channel"),
        ("T1008", "Fallback Channels"),
        ("T1105", "Ingress Tool Transfer"),
        ("T1104", "Multi-Stage Channels"),
        ("T1095", "Non-Application Layer Protocol"),
        ("T1571", "Non-Standard Port"),
        ("T1572", "Protocol Tunneling"),
        ("T1090", "Proxy"),
        ("T1219", "Remote Access Software"),
        ("T1205", "Traffic Signaling"),
        ("T1102", "Web Service"),
    ],
    "Impact": [
        ("T1531", "Account Access Removal"),
        ("T1485", "Data Destruction"),
        ("T1486", "Data Encrypted for Impact"),
        ("T1565", "Data Manipulation"),
        ("T1491", "Defacement"),
        ("T1561", "Disk Wipe"),
        ("T1499", "Endpoint Denial of Service"),
        ("T1495", "Firmware Corruption"),
        ("T1490", "Inhibit System Recovery"),
        ("T1498", "Network Denial of Service"),
        ("T1496", "Resource Hijacking"),
        ("T1489", "Service Stop"),
        ("T1529", "System Shutdown/Reboot"),
    ],
    "Resource Development": [
        ("T1583", "Acquire Infrastructure"),
        ("T1586", "Compromise Accounts"),
        ("T1584", "Compromise Infrastructure"),
        ("T1587", "Develop Capabilities"),
        ("T1585", "Establish Accounts"),
        ("T1588", "Obtain Capabilities"),
        ("T1608", "Stage Capabilities"),
    ],
    "Reconnaissance": [
        ("T1595", "Active Scanning"),
        ("T1592", "Gather Victim Host Information"),
        ("T1589", "Gather Victim Identity Information"),
        ("T1590", "Gather Victim Network Information"),
        ("T1591", "Gather Victim Org Information"),
        ("T1598", "Phishing for Information"),
        ("T1597", "Search Closed Sources"),
        ("T1596", "Search Open Technical Databases"),
        ("T1593", "Search Open Websites/Domains"),
        ("T1594", "Search Victim-Owned Websites"),
    ],
}

DEFAULT_TECHNIQUE = ("T1059", "Command and Scripting Interpreter", "Execution")


def classify_task(task_type: str):
    if task_type in TASK_TECHNIQUE_MAP:
        return TASK_TECHNIQUE_MAP[task_type]
    for prefix, tid, tname, tactic in TASK_PREFIX_MAP:
        if task_type.startswith(prefix):
            return (tid, tname, tactic)
    return DEFAULT_TECHNIQUE


def build_coverage(tasks):
    """Build per-tactic coverage counts and per-technique task counts."""
    tactic_counts = defaultdict(int)
    technique_counts = defaultdict(int)
    task_type_counts = defaultdict(int)

    for t in tasks:
        ttype = t.get("type", "") or ""
        if not ttype and t.get("command"):
            ttype = t["command"].split()[0]
        if not ttype:
            continue
        tid, tname, tactic = classify_task(ttype)
        tactic_counts[tactic] += 1
        technique_counts[f"{tid}: {tname}"] += 1
        task_type_counts[ttype] += 1

    return dict(tactic_counts), dict(technique_counts), dict(task_type_counts)


def compute_tactic_coverage_pct(tactic_name, tactic_counts):
    """Simple: 100 if any tasks hit this tactic, 0 otherwise."""
    return 100.0 if tactic_counts.get(tactic_name, 0) > 0 else 0.0


def find_gaps(tactic_counts):
    """Find tactics with no coverage and their recommended techniques."""
    gaps = {}
    for tid, tname in TACTICS_DEDUPED:
        if tactic_counts.get(tname, 0) == 0:
            gaps[tname] = TACTIC_RECOMMENDATIONS.get(tname, [])
    return gaps


def cell_color(pct):
    if pct == 0:
        return "#e74c3c", "#ffffff"   # red bg, white text
    if pct < 50:
        return "#f39c12", "#000000"   # yellow bg, black text
    return "#27ae60", "#ffffff"       # green bg, white text


def generate_html(title, tasks, tactic_counts, technique_counts, task_type_counts):
    total_tasks = len(tasks)
    gaps = find_gaps(tactic_counts)
    overall_pct = 0.0
    if TACTICS_DEDUPED:
        covered = sum(1 for _, tn in TACTICS_DEDUPED if tactic_counts.get(tn, 0) > 0)
        overall_pct = covered / len(TACTICS_DEDUPED) * 100

    # ── tactical heatmap (CSS grid) ────────────────────────────
    heatmap_cells = []
    for tid, tname in TACTICS_DEDUPED:
        pct = compute_tactic_coverage_pct(tname, tactic_counts)
        bg, fg = cell_color(pct)
        count = tactic_counts.get(tname, 0)
        heatmap_cells.append(
            f'<div class="heat-cell" style="background:{bg};color:{fg}">'
            f'<span class="heat-pct">{pct:.0f}%</span>'
            f'<span class="heat-label">{tname}</span>'
            f'<span class="heat-id">{tid}</span>'
            f'<span class="heat-count">{count} tasks</span>'
            f'</div>'
        )

    # ── technique table ────────────────────────────────────────
    tech_rows = ""
    for tkey, count in sorted(technique_counts.items(), key=lambda x: -x[1]):
        pct = count / total_tasks * 100 if total_tasks else 0
        bar_w = min(pct, 100)
        tech_rows += (
            f'<tr><td>{tkey}</td><td>{count}</td><td>{pct:.1f}%</td>'
            f'<td><div class="bar"><div class="bar-fill" style="width:{bar_w}%"></div></div></td></tr>\n'
        )

    # ── all known techniques table ─────────────────────────────
    all_techniques_rows = ""
    covered_techs = set(technique_counts.keys())
    for tactic_name in [tn for _, tn in TACTICS_DEDUPED]:
        techs = KNOWN_TECHNIQUES.get(tactic_name, [])
        for tid_t, tname_t in techs:
            key = f"{tid_t}: {tname_t}"
            status_cls = "status-covered" if key in covered_techs else "status-uncovered"
            status_label = "Covered" if key in covered_techs else "Not Covered"
            cnt = technique_counts.get(key, 0)
            all_techniques_rows += (
                f'<tr><td>{tactic_name}</td><td>{tid_t}</td><td>{tname_t}</td>'
                f'<td class="{status_cls}">{status_label}</td><td>{cnt}</td></tr>\n'
            )

    # ── gap analysis ───────────────────────────────────────────
    gap_html = ""
    for tname, recs in gaps.items():
        tid = next((ti for ti, tn in TACTICS_DEDUPED if tn == tname), "?")
        rec_items = "".join(f"<li>{r}</li>" for r in recs)
        gap_html += (
            f'<div class="gap-block">'
            f'<h4 class="gap-title">{tname} ({tid})</h4>'
            f'<p>No tasks mapped to this tactic.</p>'
            f'<ul>{rec_items}</ul>'
            f'</div>\n'
        )

    # ── task breakdown ─────────────────────────────────────────
    breakdown_rows = ""
    for ttype, cnt in sorted(task_type_counts.items(), key=lambda x: -x[1]):
        breakdown_rows += f"<tr><td>{ttype}</td><td>{cnt}</td></tr>\n"

    # ── overall coverage bar ───────────────────────────────────
    overall_color = "#e74c3c" if overall_pct < 25 else "#f39c12" if overall_pct < 50 else "#27ae60"

    html = f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>{title}</title>
<style>
* {{ box-sizing: border-box; margin: 0; padding: 0; }}
body {{ font-family: -apple-system, 'Segoe UI', sans-serif; background: #0f1117; color: #e0e0e0; padding: 24px; }}
h1 {{ font-size: 22px; color: #fff; border-bottom: 3px solid #e94560; padding-bottom: 10px; margin-bottom: 6px; }}
h2 {{ font-size: 16px; color: #e94560; margin: 28px 0 12px; text-transform: uppercase; letter-spacing: 1px; }}
h3 {{ font-size: 14px; color: #aaa; margin-bottom: 8px; }}
.subtitle {{ color: #777; font-size: 13px; margin-bottom: 20px; }}

/* ── overall coverage bar ─────────────────── */
.overall {{ margin: 12px 0 24px; }}
.overall-track {{ background: #1e2130; border-radius: 6px; height: 28px; overflow: hidden; position: relative; }}
.overall-fill {{ height: 100%; border-radius: 6px; background: {overall_color}; transition: width .4s; display: flex; align-items: center; justify-content: center; font-weight: 700; font-size: 13px; color: #fff; min-width: 48px; }}
.overall-label {{ font-size: 12px; color: #888; margin-top: 4px; }}

/* ── heatmap grid ─────────────────────────── */
.heat-grid {{ display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 10px; margin: 12px 0; }}
.heat-cell {{ border-radius: 8px; padding: 14px 10px; text-align: center; display: flex; flex-direction: column; gap: 3px; transition: transform .15s; cursor: default; }}
.heat-cell:hover {{ transform: scale(1.04); }}
.heat-pct {{ font-size: 26px; font-weight: 700; }}
.heat-label {{ font-size: 12px; font-weight: 600; }}
.heat-id {{ font-size: 10px; opacity: .7; }}
.heat-count {{ font-size: 10px; opacity: .6; margin-top: 2px; }}

/* ── tables ───────────────────────────────── */
table {{ width: 100%; border-collapse: collapse; margin: 8px 0 20px; font-size: 13px; }}
th {{ background: #1a1d2e; color: #e94560; padding: 8px 10px; text-align: left; border-bottom: 2px solid #2a2d3e; }}
td {{ padding: 6px 10px; border-bottom: 1px solid #1e2130; }}
tr:hover {{ background: #1a1d2e; }}
.status-covered {{ color: #27ae60; font-weight: 600; }}
.status-uncovered {{ color: #e74c3c; font-weight: 600; }}

/* ── bar chart ────────────────────────────── */
.bar {{ background: #1e2130; border-radius: 4px; height: 10px; overflow: hidden; }}
.bar-fill {{ height: 100%; background: #0f3460; border-radius: 4px; }}

/* ── gap analysis ─────────────────────────── */
.gap-block {{ background: #1a1d2e; border-left: 4px solid #e74c3c; border-radius: 0 6px 6px 0; padding: 14px 18px; margin: 10px 0; }}
.gap-title {{ color: #e74c3c; font-size: 14px; margin-bottom: 6px; }}
.gap-block p {{ color: #999; font-size: 12px; margin-bottom: 6px; }}
.gap-block ul {{ padding-left: 18px; }}
.gap-block li {{ color: #ccc; font-size: 12px; margin: 3px 0; }}

/* ── stats row ────────────────────────────── */
.stats-row {{ display: flex; gap: 16px; flex-wrap: wrap; margin: 12px 0 24px; }}
.stat-card {{ flex: 1; min-width: 120px; background: #1a1d2e; border-radius: 8px; padding: 16px; text-align: center; }}
.stat-card .num {{ font-size: 28px; font-weight: 700; color: #e94560; }}
.stat-card .lbl {{ font-size: 11px; color: #777; margin-top: 4px; text-transform: uppercase; letter-spacing: .5px; }}
</style></head><body>

<h1>{title}</h1>
<p class="subtitle">Generated: {datetime.now().strftime("%Y-%m-%d %H:%M:%S")} | Total Tasks: {total_tasks} | Techniques Observed: {len(technique_counts)} | Tactics Covered: {sum(1 for _, tn in TACTICS_DEDUPED if tactic_counts.get(tn, 0) > 0)}/{len(TACTICS_DEDUPED)}</p>

<div class="stats-row">
  <div class="stat-card"><div class="num">{total_tasks}</div><div class="lbl">Total Tasks</div></div>
  <div class="stat-card"><div class="num">{len(technique_counts)}</div><div class="lbl">Techniques Hit</div></div>
  <div class="stat-card"><div class="num">{len(TACTICS_DEDUPED)}</div><div class="lbl">Tactics Tracked</div></div>
  <div class="stat-card"><div class="num">{sum(1 for _, tn in TACTICS_DEDUPED if tactic_counts.get(tn, 0) > 0)}</div><div class="lbl">Tactics Covered</div></div>
  <div class="stat-card"><div class="num">{len(gaps)}</div><div class="lbl">Gaps Identified</div></div>
</div>

<h2>Overall Coverage</h2>
<div class="overall">
  <div class="overall-track"><div class="overall-fill" style="width:{overall_pct:.1f}%">{overall_pct:.1f}%</div></div>
  <div class="overall-label">{sum(1 for _, tn in TACTICS_DEDUPED if tactic_counts.get(tn, 0) > 0)} of {len(TACTICS_DEDUPED)} tactics have at least one task</div>
</div>

<h2>Tactical Heatmap</h2>
<div class="heat-grid">
{"".join(heatmap_cells)}
</div>

<h2>Techniques Observed</h2>
<table><thead><tr><th>Technique</th><th>Count</th><th>% of Tasks</th><th>Relative</th></tr></thead>
<tbody>
{tech_rows}
</tbody></table>

<h2>Full ATT&CK Coverage Matrix</h2>
<table><thead><tr><th>Tactic</th><th>Technique ID</th><th>Technique Name</th><th>Status</th><th>Task Count</th></tr></thead>
<tbody>
{all_techniques_rows}
</tbody></table>

<h2>Gap Analysis — Uncovered Tactics ({len(gaps)})</h2>
{gap_html if gap_html else '<p style="color:#27ae60;font-weight:600;">All tactics have at least some coverage.</p>'}

<h2>Task Type Breakdown</h2>
<table><thead><tr><th>Task Type</th><th>Count</th></tr></thead>
<tbody>
{breakdown_rows}
</tbody></table>

</body></html>"""
    return html


def generate_markdown(title, tasks, tactic_counts, technique_counts, task_type_counts):
    total_tasks = len(tasks)
    gaps = find_gaps(tactic_counts)
    covered = sum(1 for _, tn in TACTICS_DEDUPED if tactic_counts.get(tn, 0) > 0)
    overall_pct = covered / len(TACTICS_DEDUPED) * 100 if TACTICS_DEDUPED else 0

    lines = [
        f"# {title}",
        f"*Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')} | Tasks: {total_tasks} | Techniques: {len(technique_counts)} | Tactics: {covered}/{len(TACTICS_DEDUPED)} ({overall_pct:.1f}%)*",
        "",
        "## Tactical Heatmap",
        "| Tactic | ID | Coverage | Tasks |",
        "|---|---|---|---|",
    ]
    for tid, tname in TACTICS_DEDUPED:
        pct = compute_tactic_coverage_pct(tname, tactic_counts)
        count = tactic_counts.get(tname, 0)
        icon = "GREEN" if pct == 100 else "YELLOW" if pct > 0 else "RED"
        lines.append(f"| {tname} | {tid} | {icon} {pct:.0f}% | {count} |")

    lines += ["", "## Techniques Observed", "| Technique | Count | % Tasks |", "|---|---|---|"]
    for tkey, count in sorted(technique_counts.items(), key=lambda x: -x[1]):
        pct = count / total_tasks * 100 if total_tasks else 0
        lines.append(f"| {tkey} | {count} | {pct:.1f}% |")

    if gaps:
        lines += ["", f"## Gap Analysis — {len(gaps)} Uncovered Tactic(s)"]
        for tname, recs in gaps.items():
            tid = next((ti for ti, tn in TACTICS_DEDUPED if tn == tname), "?")
            lines.append(f"### {tname} ({tid})")
            lines.append("No tasks mapped to this tactic.\n**Recommendations:**")
            for r in recs:
                lines.append(f"- {r}")
            lines.append("")

    lines += ["", "## Task Type Breakdown", "| Type | Count |", "|---|---|"]
    for ttype, cnt in sorted(task_type_counts.items(), key=lambda x: -x[1]):
        lines.append(f"| {ttype} | {cnt} |")

    return "\n".join(lines)


def generate_json(title, tasks, tactic_counts, technique_counts, task_type_counts):
    gaps = find_gaps(tactic_counts)
    covered = sum(1 for _, tn in TACTICS_DEDUPED if tactic_counts.get(tn, 0) > 0)
    overall_pct = covered / len(TACTICS_DEDUPED) * 100 if TACTICS_DEDUPED else 0
    return json.dumps({
        "title": title,
        "generated": datetime.now().isoformat(),
        "total_tasks": len(tasks),
        "overall_coverage_pct": round(overall_pct, 1),
        "tactics": {
            tn: {"id": tid, "covered": tactic_counts.get(tn, 0) > 0, "task_count": tactic_counts.get(tn, 0)}
            for tid, tn in TACTICS_DEDUPED
        },
        "techniques": technique_counts,
        "gaps": gaps,
        "task_types": task_type_counts,
    }, indent=2)


def main():
    data_input = read_stdin()
    params = data_input.get("params", {})

    fmt = params.get("format", "html")
    title = params.get("title", "ATT\u0026CK Coverage Dashboard")

    try:
        db = Database()
    except FileNotFoundError as e:
        print(json.dumps({"title": title, "format": fmt, "content": f"Error: {e}"}))
        return

    try:
        tasks = db.all_tasks()
        tactic_counts, technique_counts, task_type_counts = build_coverage(tasks)

        if fmt == "json":
            content = generate_json(title, tasks, tactic_counts, technique_counts, task_type_counts)
        elif fmt == "markdown":
            content = generate_markdown(title, tasks, tactic_counts, technique_counts, task_type_counts)
        else:
            content = generate_html(title, tasks, tactic_counts, technique_counts, task_type_counts)
            fmt = "html"

        write_report(title, content, fmt)
    finally:
        db.close()


if __name__ == "__main__":
    main()
