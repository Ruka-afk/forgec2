#!/usr/bin/env python3
import json, os, sys
from datetime import datetime
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_report

HIGH_VALUE_USERS = {
    "administrator", "admin", "root", "sa", "krbtgt", "domain admins",
    "enterprise admins", "schema admins", "backup operator", "server operator",
}

SERVICE_MAP = {
    "ssh": "Remote SSH access",
    "rdp": "Remote Desktop access",
    "smb": "File share access",
    "wmi": "Remote command execution",
    "winrm": "PowerShell remoting",
    "mssql": "SQL Server access",
    "mysql": "MySQL access",
    "ldap": "Directory access",
    "http": "Web application access",
}


def _extract_domain(cred):
    user = cred.get("username", "")
    if "\\" in user:
        return user.split("\\")[0]
    if "@" in user:
        return user.split("@")[1]
    return ""


def _is_high_value(cred):
    user = (cred.get("username", "") or "").lower()
    for hv in HIGH_VALUE_USERS:
        if hv in user:
            return True
    return False


def generate_html(data, title):
    agents = data["agents"]
    creds = data["credentials"]
    paths = data["paths"]
    summary = data["summary"]

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
.stat {{ display: inline-block; background: #e8f4f8; border-left: 4px solid #0f3460; padding: 10px 15px; margin: 5px; border-radius: 4px; }}
.stat .num {{ font-size: 24px; font-weight: bold; color: #e94560; }}
.stat .label {{ font-size: 12px; color: #666; }}
.badge {{ display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: bold; }}
.critical {{ background: #dc3545; color: white; }}
.high {{ background: #fd7e14; color: white; }}
.medium {{ background: #ffc107; color: #333; }}
.low {{ background: #28a745; color: white; }}
.path-box {{ background: #f0f4f8; border-left: 4px solid #0f3460; padding: 12px 16px; margin: 10px 0; border-radius: 0 6px 6px 0; }}
.path-box .method {{ font-weight: bold; color: #e94560; }}
.path-box .risk {{ float: right; }}
.arrow {{ color: #e94560; font-weight: bold; margin: 0 8px; }}
</style></head><body>
<h1>{title}</h1>
<p>Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}</p>

<h2>Summary</h2>
<div>
  <div class="stat"><div class="num">{summary['total_credentials']}</div><div class="label">Total Credentials</div></div>
  <div class="stat"><div class="num">{summary['high_value']}</div><div class="label">High-Value</div></div>
  <div class="stat"><div class="num">{summary['unique_domains']}</div><div class="label">Unique Domains</div></div>
  <div class="stat"><div class="num">{summary['total_paths']}</div><div class="label">Attack Paths</div></div>
</div>

<h2>Credentials Collected</h2>
<table><tr><th>Username</th><th>Type</th><th>Source</th><th>Domain</th><th>High-Value</th></tr>"""
    for c in creds[:50]:
        domain = _extract_domain(c)
        hv = _is_high_value(c)
        hv_cls = "critical" if hv else ""
        html += f"""<tr><td>{c.get('username','')}</td><td>{c.get('type','')}</td>
<td>{c.get('source','')}</td><td>{domain}</td>
<td>{"<span class='badge critical'>HIGH VALUE</span>" if hv else ""}</td></tr>"""
    html += "</table>"

    html += "<h2>Attack Paths</h2>"
    for p in sorted(paths, key=lambda x: -x.get("score", 0))[:30]:
        risk_cls = {"critical": "critical", "high": "high", "medium": "medium"}.get(p.get("risk", ""), "low")
        html += f"""<div class="path-box">
<span class="badge {risk_cls} risk">{p.get('risk','').upper()}</span>
<strong>{p.get('source_agent','')}</strong>
<span class="arrow">&rarr;</span>
<span class="method">{p.get('method','')}</span>
<span class="arrow">&rarr;</span>
<strong>{p.get('target','')}</strong>
<br><small>Credential: {p.get('credential_used','')} | Confidence: {p.get('confidence',0):.0%}</small>
</div>"""

    html += "</body></html>"
    return html


def generate_markdown(data, title):
    summary = data["summary"]
    creds = data["credentials"]
    paths = data["paths"]
    lines = [
        f"# {title}",
        f"*Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}*",
        "",
        "## Summary",
        f"- **Total Credentials:** {summary['total_credentials']}",
        f"- **High-Value:** {summary['high_value']}",
        f"- **Unique Domains:** {summary['unique_domains']}",
        f"- **Attack Paths:** {summary['total_paths']}",
        "",
        "## Credentials",
        "| Username | Type | Domain | High-Value |",
        "|---|---|---|---|",
    ]
    for c in creds[:30]:
        domain = _extract_domain(c)
        hv = "YES" if _is_high_value(c) else ""
        lines.append(f"| {c.get('username','')} | {c.get('type','')} | {domain} | {hv} |")

    lines += ["", "## Top Attack Paths", "| Source | Method | Target | Risk |", "|---|---|---|---|"]
    for p in sorted(paths, key=lambda x: -x.get("score", 0))[:20]:
        lines.append(f"| {p.get('source_agent','')} | {p.get('method','')} | {p.get('target','')} | {p.get('risk','')} |")
    return "\n".join(lines)


def build_attack_paths(agents, creds):
    paths = []
    domain_agents = {}
    for a in agents:
        domain = (a.get("domain") or "").lower()
        if domain:
            domain_agents.setdefault(domain, []).append(a)

    for cred in creds:
        username = (cred.get("username") or "").lower()
        domain = _extract_domain(cred).lower()
        cred_type = (cred.get("type") or "").lower()
        hv = _is_high_value(cred)

        for agent in agents:
            agent_domain = (agent.get("domain") or "").lower()
            agent_os = (agent.get("os") or "").lower()
            method = ""
            confidence = 0.5

            if domain and agent_domain == domain:
                method = "domain authentication"
                confidence = 0.9 if hv else 0.6
            elif cred_type in ("hash", "ntlm"):
                method = "pass-the-hash"
                confidence = 0.8
            elif cred_type == "password":
                if "windows" in agent_os:
                    method = "RDP/WinRM"
                elif "linux" in agent_os:
                    method = "SSH"
                confidence = 0.7
            elif cred_type in ("api_key", "token"):
                method = "API authentication"
                confidence = 0.6

            if method:
                risk = "critical" if hv and confidence > 0.8 else "high" if confidence > 0.7 else "medium" if confidence > 0.5 else "low"
                paths.append({
                    "source_agent": agent.get("hostname", agent["id"][:8]),
                    "target": f"{agent.get('hostname', '')} ({agent.get('ip', '')})",
                    "credential_used": f"{cred.get('username', '')} ({cred.get('type', '')})",
                    "method": method,
                    "confidence": confidence,
                    "risk": risk,
                    "score": confidence * (2 if hv else 1),
                })
    return paths


def main():
    data_input = read_stdin()
    params = data_input.get("params", {})
    config = data_input.get("config", {})
    fmt = params.get("format", "html")
    title = params.get("title", "Credential Attack Paths")

    try:
        db = Database()
    except FileNotFoundError as e:
        print(json.dumps({"title": title, "format": fmt, "content": f"Error: {e}"}))
        return

    try:
        agents = db.all_agents()
        creds = db.all_credentials()
        paths = build_attack_paths(agents, creds)

        domains = set()
        high_value = 0
        for c in creds:
            d = _extract_domain(c)
            if d:
                domains.add(d.lower())
            if _is_high_value(c):
                high_value += 1

        summary = {
            "total_credentials": len(creds),
            "high_value": high_value,
            "unique_domains": len(domains),
            "total_paths": len(paths),
        }

        report_data = {
            "agents": agents,
            "credentials": creds,
            "paths": paths,
            "summary": summary,
        }

        if fmt == "json":
            content = json.dumps(report_data, indent=2, default=str)
        elif fmt == "markdown":
            content = generate_markdown(report_data, title)
        else:
            content = generate_html(report_data, title)
            fmt = "html"

        write_report(title, content, fmt)
    finally:
        db.close()


if __name__ == "__main__":
    main()
