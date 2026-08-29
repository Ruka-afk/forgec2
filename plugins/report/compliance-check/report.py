#!/usr/bin/env python3
"""Compliance Check Report plugin — maps findings to CIS Benchmark and NIST 800-53 controls."""

import json
import os
import sys
from datetime import datetime

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_report


CIS_CONTROLS = {
    "CIS 1": {
        "name": "Inventory and Control of Enterprise Assets",
        "nist": "CM-8",
        "description": "Maintain an accurate and up-to-date inventory of all enterprise assets",
    },
    "CIS 2": {
        "name": "Inventory and Control of Software Assets",
        "nist": "CM-11",
        "description": "Actively manage all software on the network so that only authorized software is installed",
    },
    "CIS 3": {
        "name": "Continuous Vulnerability Management",
        "nist": "RA-5",
        "description": "Continuously acquire, assess, and take action on new information to identify vulnerabilities",
    },
    "CIS 4": {
        "name": "Controlled Use of Administrator Privileges",
        "nist": "AC-6",
        "description": "Processes and procedures to manage the lifecycle of administrator accounts",
    },
    "CIS 5": {
        "name": "Secure Configuration for Enterprise Assets and Software",
        "nist": "CM-2",
        "description": "Establish and maintain the secure configuration of enterprise assets and software",
    },
    "CIS 6": {
        "name": "Account Management",
        "nist": "AC-2",
        "description": "Manage the lifecycle of system and administrator accounts",
    },
    "CIS 7": {
        "name": "Continuous Vulnerability Management — Access Control",
        "nist": "AC-5",
        "description": "Restrict administrative privileges to only those who need them",
    },
    "CIS 8": {
        "name": "Audit Log Management",
        "nist": "AU-6",
        "description": "Collect, alert, review, and retain audit logs to detect, understand, or recover from attacks",
    },
    "CIS 10": {
        "name": "Data Recovery",
        "nist": "CP-9",
        "description": "Establish and maintain data recovery practices for enterprise assets",
    },
    "CIS 11": {
        "name": "Network Infrastructure Management",
        "nist": "SC-7",
        "description": "Establish, implement, and actively manage network devices to protect the network",
    },
    "CIS 13": {
        "name": "Network Monitoring and Defense",
        "nist": "SI-4",
        "description": "Operate processes to collect, detect, and respond to network intrusions",
    },
}

NIST_CONTROLS = {
    "AC": {"name": "Access Control", "description": "Policy and procedures for controlling access to information systems"},
    "AU": {"name": "Audit and Accountability", "description": "Procedures for audit logging and accountability"},
    "CM": {"name": "Configuration Management", "description": "Procedures for managing configuration of information systems"},
    "IA": {"name": "Identification and Authentication", "description": "Procedures for identifying and authenticating users"},
    "RA": {"name": "Risk Assessment", "description": "Procedures for assessing risks to organizational operations"},
    "SC": {"name": "System and Communications Protection", "description": "Procedures for protecting system and communications integrity"},
}


def evaluate_cis(agents, tasks, creds, listeners, hosts):
    """Evaluate all CIS controls and return results dict."""
    results = {}

    # CIS 1: Asset Inventory
    total_assets = len(agents) + len(hosts)
    online_agents = sum(1 for a in agents if (a.get("status") or "").lower() == "online")
    if total_assets > 0:
        has_inventory = True
        status = "pass" if total_assets > 0 else "fail"
    else:
        has_inventory = False
        status = "partial"
    findings = [f"Total assets tracked: {total_assets}"]
    if agents:
        findings.append(f"Agents: {len(agents)} total, {online_agents} online")
    if hosts:
        findings.append(f"Network hosts discovered: {len(hosts)}")
    if not agents and not hosts:
        findings.append("No assets recorded — inventory process not established")
    results["CIS 1"] = {"status": status, "findings": findings}

    # CIS 2: Software Inventory
    software_types = {}
    for t in tasks:
        ttype = (t.get("type") or "").lower()
        if ttype in ("shell", "powershell", "execute", "list_software", "software_inventory"):
            software_types[ttype] = software_types.get(ttype, 0) + 1
    findings = []
    if software_types:
        findings.append(f"Task types indicating software discovery: {software_types}")
        status = "pass"
    else:
        findings.append("No software inventory tasks detected")
        status = "partial"
    results["CIS 2"] = {"status": status, "findings": findings}

    # CIS 3: Continuous Vulnerability Management
    legacy_keywords = ["xp", "vista", "2003", "2008", "windows 7"]
    legacy_agents = []
    for a in agents:
        os_name = (a.get("os") or "").lower()
        for kw in legacy_keywords:
            if kw in os_name:
                legacy_agents.append(a.get("hostname") or a.get("id") or "unknown")
                break
    findings = []
    if legacy_agents:
        findings.append(f"Agents running legacy/outdated OS: {legacy_agents}")
        status = "fail"
    elif agents:
        findings.append("No agents running legacy OS detected")
        status = "pass"
    else:
        findings.append("No agents to evaluate")
        status = "not_applicable"
    results["CIS 3"] = {"status": status, "findings": findings}

    # CIS 4: Admin Privileges
    admin_agents = []
    for a in agents:
        username = (a.get("username") or "").lower()
        if "admin" in username or "system" in username or "root" in username:
            admin_agents.append(a.get("username") or "unknown")
    findings = []
    if admin_agents:
        findings.append(f"Agents running with elevated privileges: {admin_agents}")
        findings.append("Review whether all privileged access is necessary")
        status = "fail"
    elif agents:
        findings.append("No agents detected running with admin/root privileges")
        status = "pass"
    else:
        findings.append("No agents to evaluate")
        status = "not_applicable"
    results["CIS 4"] = {"status": status, "findings": findings}

    # CIS 5: Secure Configuration
    findings = []
    issues = 0
    for l in listeners:
        protocol = (l.get("protocol") or "").lower()
        ssl_enabled = l.get("ssl_enabled")
        if protocol == "http" and not ssl_enabled:
            findings.append(f"Listener '{l.get('name', 'unknown')}' uses unencrypted HTTP")
            issues += 1
    if issues == 0:
        findings.append("All listeners appear to use secure configurations")
        status = "pass"
    else:
        status = "fail"
    if not listeners:
        findings.append("No listeners configured")
        status = "partial"
    results["CIS 5"] = {"status": status, "findings": findings}

    # CIS 6: Account Management
    cleartext_types = {"password", "plaintext", "cleartext", "raw", ""}
    cleartext_creds = []
    high_value_accounts = {"admin", "administrator", "krbtgt", "service", "sql", "enterprise", "sa"}
    exposed_high_value = []
    for c in creds:
        ctype = (c.get("type") or "").lower()
        if ctype in cleartext_types:
            cleartext_creds.append(c.get("username") or "unknown")
        user = (c.get("username") or "").lower()
        for hv in high_value_accounts:
            if hv in user:
                exposed_high_value.append(c.get("username") or "unknown")
                break
    findings = []
    if cleartext_creds:
        findings.append(f"Credentials stored in cleartext: {cleartext_creds}")
        findings.append("Rotate these credentials immediately")
    if exposed_high_value:
        findings.append(f"High-value accounts with exposed credentials: {exposed_high_value}")
    if not cleartext_creds and not exposed_high_value:
        findings.append("No cleartext credentials or exposed high-value accounts detected")
        status = "pass"
    elif cleartext_creds:
        status = "fail"
    else:
        status = "partial"
    results["CIS 6"] = {"status": status, "findings": findings}

    # CIS 7: Access Control — Least Privilege
    findings = []
    cred_count = len(creds)
    if cred_count > 0:
        reused = 0
        by_user = {}
        for c in creds:
            user = (c.get("username") or "").lower()
            by_user.setdefault(user, []).append(c)
        for user, entries in by_user.items():
            if len(entries) > 1:
                reused += 1
        if reused > 0:
            findings.append(f"Accounts with reused credentials: {reused}")
            status = "partial"
        else:
            findings.append("No credential reuse detected")
            status = "pass"
        findings.append(f"Total credentials tracked: {cred_count}")
    else:
        findings.append("No credentials to evaluate")
        status = "not_applicable"
    results["CIS 7"] = {"status": status, "findings": findings}

    # CIS 8: Audit Log Management
    findings = []
    audit_logs = []
    try:
        db_ref = Database()
        audit_logs = db_ref.all_audit_logs()
        db_ref.close()
    except Exception as exc:
        print(json.dumps({"level": "error", "message": f"Plugin error: failed to read audit logs for CIS 8 evaluation: {exc}"}), file=sys.stderr)
    if audit_logs:
        findings.append(f"Audit log entries present: {len(audit_logs)}")
        actions = {}
        for log in audit_logs[:500]:
            action = log.get("action") or "unknown"
            actions[action] = actions.get(action, 0) + 1
        if actions:
            top_actions = sorted(actions.items(), key=lambda x: -x[1])[:5]
            findings.append(f"Top logged actions: {dict(top_actions)}")
        status = "pass"
    else:
        findings.append("No audit log entries found — logging may not be enabled")
        status = "fail"
    results["CIS 8"] = {"status": status, "findings": findings}

    # CIS 10: Data Recovery
    findings = []
    findings.append("Evaluate backup and recovery procedures outside agent scope")
    findings.append("Verify database backups for the ForgeC2 server")
    findings.append("Ensure configuration and credential stores are backed up")
    status = "partial"
    results["CIS 10"] = {"status": status, "findings": findings}

    # CIS 11: Network Infrastructure Management
    findings = []
    host_count = len(hosts)
    if host_count > 0:
        open_port_hosts = 0
        for h in hosts:
            ports = h.get("open_ports") or ""
            if isinstance(ports, str) and ports.strip():
                open_port_hosts += 1
            elif isinstance(ports, list) and ports:
                open_port_hosts += 1
        findings.append(f"Network hosts discovered: {host_count}")
        if open_port_hosts > 0:
            findings.append(f"Hosts with open ports: {open_port_hosts}")
            status = "partial"
        else:
            status = "pass"
    else:
        findings.append("No network hosts discovered — network visibility limited")
        status = "partial"
    results["CIS 11"] = {"status": status, "findings": findings}

    # CIS 13: Network Monitoring and Defense
    findings = []
    recon_tasks = 0
    for t in tasks:
        ttype = (t.get("type") or "").lower()
        if ttype in ("screenshot", "keylog", "sharphound", "bloodhound", "netstat", "arp", "nmap", "recon"):
            recon_tasks += 1
    findings.append(f"Reconnaissance/monitoring tasks executed: {recon_tasks}")
    if recon_tasks > 0:
        findings.append("Active monitoring and recon capability present")
        status = "pass"
    else:
        findings.append("No network monitoring or defense tasks detected")
        status = "partial"
    results["CIS 13"] = {"status": status, "findings": findings}

    return results


def evaluate_nist(agents, tasks, creds, listeners, hosts):
    """Map CIS results to NIST 800-53 controls."""
    cis_results = evaluate_cis(agents, tasks, creds, listeners, hosts)

    nist_map = {
        "AC": ["CIS 4", "CIS 6", "CIS 7"],
        "AU": ["CIS 8"],
        "CM": ["CIS 1", "CIS 2", "CIS 5"],
        "IA": ["CIS 6"],
        "RA": ["CIS 3", "CIS 10"],
        "SC": ["CIS 5", "CIS 11", "CIS 13"],
    }

    results = {}
    for nist_id, nist_info in NIST_CONTROLS.items():
        mapped_cis = nist_map.get(nist_id, [])
        all_findings = []
        statuses = []
        for cis_id in mapped_cis:
            if cis_id in cis_results:
                cis = cis_results[cis_id]
                statuses.append(cis["status"])
                all_findings.append(f"[{cis_id}] {cis['name']}: {', '.join(cis['findings'][:2])}")

        if not statuses:
            status = "not_applicable"
        elif all(s == "pass" for s in statuses):
            status = "pass"
        elif any(s == "fail" for s in statuses):
            status = "fail"
        else:
            status = "partial"

        results[nist_id] = {
            "name": nist_info["name"],
            "description": nist_info["description"],
            "status": status,
            "findings": all_findings if all_findings else [nist_info["description"]],
        }

    return results


def compute_score(results):
    """Compute compliance score from control results."""
    total = len(results)
    passed = sum(1 for r in results.values() if r["status"] == "pass")
    partial = sum(1 for r in results.values() if r["status"] == "partial")
    if total == 0:
        return 0, 0, 0, 0
    score = round((passed + partial * 0.5) / total * 100)
    return score, total, passed, partial


def remediation(control_id, status, findings):
    """Generate remediation recommendations for failed controls."""
    recs = {
        "CIS 1": "Deploy automated asset discovery agents across all network segments. Maintain a centralized CMDB.",
        "CIS 2": "Implement software inventory scanning tools. Restrict unauthorized software installation.",
        "CIS 3": "Deploy vulnerability scanning tools. Patch or isolate systems running legacy OS versions.",
        "CIS 4": "Enforce principle of least privilege. Remove unnecessary admin accounts. Enable MFA for all admin access.",
        "CIS 5": "Enable TLS on all listeners. Audit listener configurations against security baselines.",
        "CIS 6": "Rotate cleartext credentials immediately. Store secrets in a vault. Disable unused accounts.",
        "CIS 7": "Enforce unique passwords per account. Implement credential rotation policies.",
        "CIS 8": "Enable audit logging on all systems. Configure log retention and alerting.",
        "CIS 10": "Establish automated backup procedures. Test recovery regularly.",
        "CIS 11": "Inventory all network devices. Implement network segmentation and monitoring.",
        "CIS 13": "Deploy IDS/IPS sensors. Enable DNS monitoring and network traffic analysis.",
    }
    if status == "pass":
        return None
    base = recs.get(control_id, "Review and address findings for this control.")
    if status == "partial":
        return f"{base} (Currently partial compliance — address gaps identified in findings)."
    return f"NON-COMPLIANT. {base}"


def generate_html(title, framework, cis_results, nist_results, cis_score, nist_score, overall_score):
    """Generate HTML compliance report."""
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    status_colors = {
        "pass": "#28a745",
        "fail": "#dc3545",
        "partial": "#ffc107",
        "not_applicable": "#6c757d",
    }
    status_bg = {
        "pass": "#d4edda",
        "fail": "#f8d7da",
        "partial": "#fff3cd",
        "not_applicable": "#e2e3e5",
    }

    def score_color(s):
        if s >= 80:
            return "#28a745"
        if s >= 60:
            return "#ffc107"
        return "#dc3545"

    def score_bg(s):
        if s >= 80:
            return "#d4edda"
        if s >= 60:
            return "#fff3cd"
        return "#f8d7da"

    ocolor = score_color(overall_score)
    obg = score_bg(overall_score)

    control_rows = ""
    failed_controls = []

    if framework in ("cis", "both"):
        for cid, info in CIS_CONTROLS.items():
            r = cis_results.get(cid, {"status": "not_applicable", "findings": []})
            sc = status_colors.get(r["status"], "#6c757d")
            sbg = status_bg.get(r["status"], "#e2e3e5")
            findings_html = "".join(f"<li>{f}</li>" for f in r["findings"])
            rec = remediation(cid, r["status"], r["findings"])
            rec_html = f'<div class="rec">{rec}</div>' if rec else ""
            if r["status"] in ("fail", "partial"):
                failed_controls.append((cid, info["name"], r["status"], r["findings"], rec))
            control_rows += f"""
        <tr>
          <td><strong>{cid}</strong></td>
          <td>{info['name']}<br><span class="framework-id">NIST: {info['nist']}</span></td>
          <td style="background:{sbg};color:{sc};font-weight:600;">{r['status'].upper().replace('_', ' ')}</td>
          <td><ul class="findings">{findings_html}</ul></td>
          <td>{rec_html}</td>
        </tr>"""

    if framework in ("nist", "both"):
        for nid, r in nist_results.items():
            info = NIST_CONTROLS[nid]
            sc = status_colors.get(r["status"], "#6c757d")
            sbg = status_bg.get(r["status"], "#e2e3e5")
            findings_html = "".join(f"<li>{f}</li>" for f in r["findings"])
            rec = remediation(nid, r["status"], r["findings"])
            rec_html = f'<div class="rec">{rec}</div>' if rec else ""
            control_rows += f"""
        <tr>
          <td><strong>{nid}</strong></td>
          <td>{info['name']}<br><span class="framework-id">{info['description']}</span></td>
          <td style="background:{sbg};color:{sc};font-weight:600;">{r['status'].upper().replace('_', ' ')}</td>
          <td><ul class="findings">{findings_html}</ul></td>
          <td>{rec_html}</td>
        </tr>"""

    summary_items = ""
    if failed_controls:
        for cid, name, status, findings, rec in failed_controls:
            sc = status_colors.get(status, "#6c757d")
            summary_items += f"""
      <div class="issue" style="border-left-color:{sc};">
        <strong>{cid}: {name}</strong> — {status.upper().replace('_', ' ')}
        <p>{'; '.join(findings[:2])}</p>
      </div>"""
    else:
        summary_items = '<div class="issue" style="border-left-color:#28a745;"><strong>No critical compliance issues found.</strong></div>'

    score_cards = ""
    if framework in ("cis", "both"):
        score_cards += f"""
      <div class="score-card" style="border-left-color:{score_color(cis_score)};">
        <div class="card-header">
          <span class="card-title">CIS Benchmark</span>
          <span class="card-score" style="color:{score_color(cis_score)};">{cis_score}%</span>
        </div>
        <div class="card-bar"><div class="card-bar-fill" style="width:{cis_score}%;background:{score_color(cis_score)};"></div></div>
      </div>"""
    if framework in ("nist", "both"):
        score_cards += f"""
      <div class="score-card" style="border-left-color:{score_color(nist_score)};">
        <div class="card-header">
          <span class="card-title">NIST 800-53</span>
          <span class="card-score" style="color:{score_color(nist_score)};">{nist_score}%</span>
        </div>
        <div class="card-bar"><div class="card-bar-fill" style="width:{nist_score}%;background:{score_color(nist_score)};"></div></div>
      </div>"""

    return f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>{title}</title>
<style>
body {{ font-family: -apple-system, sans-serif; max-width: 1100px; margin: 0 auto; padding: 20px; color: #1a1a2e; background: #fafafa; }}
h1 {{ color: #0f3460; border-bottom: 3px solid #e94560; padding-bottom: 10px; }}
h2 {{ color: #16213e; margin-top: 30px; border-bottom: 1px solid #ddd; padding-bottom: 5px; }}
.subtitle {{ color: #666; font-size: 14px; }}
.overall {{ display: flex; align-items: center; gap: 20px; background: {obg}; border: 1px solid {ocolor}; border-radius: 10px; padding: 20px 30px; margin: 15px 0; }}
.overall .big {{ font-size: 56px; font-weight: bold; color: {ocolor}; }}
.overall .meta {{ font-size: 14px; color: #444; }}
.score-cards {{ display: flex; gap: 14px; margin: 15px 0; }}
.score-card {{ background: #fff; border-left: 5px solid; border-radius: 6px; padding: 14px 16px; box-shadow: 0 1px 3px rgba(0,0,0,0.08); flex: 1; }}
.card-header {{ display: flex; justify-content: space-between; align-items: center; }}
.card-title {{ font-weight: 600; font-size: 14px; color: #16213e; }}
.card-score {{ font-size: 20px; font-weight: bold; }}
.card-bar {{ background: #e9ecef; border-radius: 4px; height: 8px; margin: 6px 0; overflow: hidden; }}
.card-bar-fill {{ height: 100%; border-radius: 4px; transition: width 0.3s; }}
table {{ width: 100%; border-collapse: collapse; margin: 15px 0; font-size: 13px; }}
th {{ background: #0f3460; color: #fff; padding: 10px 12px; text-align: left; }}
td {{ padding: 10px 12px; border-bottom: 1px solid #eee; vertical-align: top; }}
tr:hover {{ background: #f0f4ff; }}
.findings {{ list-style: none; padding: 0; margin: 0; }}
.findings li {{ margin: 2px 0; font-size: 12px; color: #444; }}
.framework-id {{ font-size: 11px; color: #888; }}
.issue {{ background: #fff; border-left: 4px solid; padding: 12px 16px; margin: 8px 0; border-radius: 4px; box-shadow: 0 1px 2px rgba(0,0,0,0.05); }}
.issue strong {{ color: #16213e; }}
.issue p {{ margin: 4px 0 0 0; font-size: 13px; color: #555; }}
.rec {{ font-size: 12px; color: #0f3460; background: #e8f0fe; padding: 6px 10px; border-radius: 4px; margin-top: 4px; }}
</style></head><body>
<h1>{title}</h1>
<p class="subtitle">Generated: {now} &mdash; Framework: {framework.upper()}</p>

<div class="overall">
  <div class="big">{overall_score}%</div>
  <div class="meta">Overall Compliance Score</div>
</div>

<h2>Framework Scores</h2>
<div class="score-cards">{score_cards}
</div>

<h2>Executive Summary</h2>
{summary_items}

<h2>Control Assessment</h2>
<table>
  <thead>
    <tr><th>Control</th><th>Description</th><th>Status</th><th>Findings</th><th>Remediation</th></tr>
  </thead>
  <tbody>{control_rows}
  </tbody>
</table>

</body></html>"""


def generate_markdown(title, framework, cis_results, nist_results, cis_score, nist_score, overall_score):
    """Generate Markdown compliance report."""
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    lines = [
        f"# {title}",
        f"*Generated: {now} — Framework: {framework.upper()}*",
        "",
        f"## Overall Compliance Score: {overall_score}%",
        "",
    ]

    if framework in ("cis", "both"):
        lines += [f"### CIS Benchmark: {cis_score}%", ""]
    if framework in ("nist", "both"):
        lines += [f"### NIST 800-53: {nist_score}%", ""]

    lines += ["## Executive Summary", ""]
    failed = []
    if framework in ("cis", "both"):
        for cid, info in CIS_CONTROLS.items():
            r = cis_results.get(cid, {"status": "not_applicable", "findings": []})
            if r["status"] in ("fail", "partial"):
                failed.append((cid, info["name"], r["status"], r["findings"]))
    if framework in ("nist", "both"):
        for nid, r in nist_results.items():
            if r["status"] in ("fail", "partial"):
                failed.append((nid, NIST_CONTROLS[nid]["name"], r["status"], r["findings"]))

    if failed:
        for cid, name, status, findings in failed:
            lines.append(f"- **{cid} ({name})**: {status.upper()} — {'; '.join(findings[:2])}")
    else:
        lines.append("- No critical compliance issues found.")
    lines.append("")

    if framework in ("cis", "both"):
        lines += ["## CIS Control Assessment", "", "| Control | Description | Status | Findings |", "|---------|-------------|--------|----------|"]
        for cid, info in CIS_CONTROLS.items():
            r = cis_results.get(cid, {"status": "not_applicable", "findings": []})
            f_list = "; ".join(r["findings"][:2])
            lines.append(f"| {cid} | {info['name']} | {r['status'].upper()} | {f_list} |")
        lines.append("")

    if framework in ("nist", "both"):
        lines += ["## NIST 800-53 Assessment", "", "| Control | Name | Status | Findings |", "|---------|------|--------|----------|"]
        for nid, r in nist_results.items():
            f_list = "; ".join(r["findings"][:2])
            lines.append(f"| {nid} | {r['name']} | {r['status'].upper()} | {f_list} |")
        lines.append("")

    lines += ["## Remediation Recommendations", ""]
    recs_shown = False
    if framework in ("cis", "both"):
        for cid, info in CIS_CONTROLS.items():
            r = cis_results.get(cid, {"status": "not_applicable", "findings": []})
            rec = remediation(cid, r["status"], r["findings"])
            if rec:
                lines.append(f"- **{cid}**: {rec}")
                recs_shown = True
    if not recs_shown:
        lines.append("- No remediation needed — all controls are compliant.")
    lines.append("")

    return "\n".join(lines)


def generate_json(title, framework, cis_results, nist_results, cis_score, nist_score, overall_score):
    """Generate JSON compliance report."""
    data = {
        "title": title,
        "generated_at": datetime.now().isoformat(),
        "framework": framework,
        "overall_score": overall_score,
        "scores": {},
        "controls": {},
    }
    if framework in ("cis", "both"):
        data["scores"]["cis"] = cis_score
        data["controls"]["cis"] = {}
        for cid, info in CIS_CONTROLS.items():
            r = cis_results.get(cid, {"status": "not_applicable", "findings": []})
            data["controls"]["cis"][cid] = {
                "name": info["name"],
                "nist_mapping": info["nist"],
                "status": r["status"],
                "findings": r["findings"],
                "remediation": remediation(cid, r["status"], r["findings"]),
            }
    if framework in ("nist", "both"):
        data["scores"]["nist"] = nist_score
        data["controls"]["nist"] = {}
        for nid, r in nist_results.items():
            data["controls"]["nist"][nid] = {
                "name": r["name"],
                "description": r["description"],
                "status": r["status"],
                "findings": r["findings"],
                "remediation": remediation(nid, r["status"], r["findings"]),
            }
    return json.dumps(data, indent=2, default=str)


def main():
    data_input = read_stdin()
    params = data_input.get("params", {})

    fmt = params.get("format", "html")
    title = params.get("title", "Compliance Check Report")
    framework = params.get("framework", "cis").lower()

    try:
        db = Database()
    except FileNotFoundError as e:
        print(json.dumps({"title": title, "format": fmt, "content": f"Error: {e}"}))
        return

    try:
        agents = db.all_agents()
        tasks = db.all_tasks()
        creds = db.all_credentials()
        listeners = db.all_listeners()
        hosts = db.all_network_hosts()

        cis_results = evaluate_cis(agents, tasks, creds, listeners, hosts)
        cis_score, cis_total, cis_passed, cis_partial = compute_score(cis_results)

        nist_results = evaluate_nist(agents, tasks, creds, listeners, hosts)
        nist_score, nist_total, nist_passed, nist_partial = compute_score(nist_results)

        if framework == "cis":
            overall_score = cis_score
        elif framework == "nist":
            overall_score = nist_score
        else:
            overall_score = round((cis_score + nist_score) / 2)

        if fmt == "json":
            content = generate_json(title, framework, cis_results, nist_results, cis_score, nist_score, overall_score)
        elif fmt == "markdown":
            content = generate_markdown(title, framework, cis_results, nist_results, cis_score, nist_score, overall_score)
        else:
            content = generate_html(title, framework, cis_results, nist_results, cis_score, nist_score, overall_score)
            fmt = "html"

        write_report(title, content, fmt)
    finally:
        db.close()


if __name__ == "__main__":
    main()
