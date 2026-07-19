#!/usr/bin/env python3
import json
import re
import sys

sys.path.insert(0, ".")
from lib.db import Database, read_stdin, write_result


def parse_net_accounts(text):
    policy = {}
    lines = text.splitlines()
    for line in lines:
        line = line.strip()
        m = re.match(r"Minimum password length:\s*(\d+)", line, re.I)
        if m:
            policy["min_length"] = int(m.group(1))
        m = re.match(r"Maximum password age\s*\((\w+)\):\s*(\d+)\s*days", line, re.I)
        if m:
            val = m.group(2)
            policy["max_age_days"] = int(val) if val.isdigit() else None
            if policy["max_age_days"] == 0:
                policy["max_age_days"] = None
        m = re.match(r"Password history kept:\s*(\d+)", line, re.I)
        if m:
            policy["history_count"] = int(m.group(1))
        m = re.match(r"Lockout threshold:\s*(\d+)", line, re.I)
        if m:
            policy["lockout_threshold"] = int(m.group(1))
        m = re.match(r"Lockout duration\s*\((\w+)\):\s*(\d+)\s*minutes", line, re.I)
        if m:
            policy["lockout_duration_min"] = int(m.group(2))
        m = re.match(r"Lockout observation window\s*\(\w+\):\s*(\d+)\s*minutes", line, re.I)
        if m:
            policy["lockout_reset_min"] = int(m.group(1))
    return policy


def parse_ad_domain_policy(text):
    policy = {}
    m = re.search(r"MinPasswordLength\s*:\s*(\d+)", text, re.I)
    if m:
        policy["min_length"] = int(m.group(1))
    m = re.search(r"MaxPasswordAge\s*:\s*\.(\d+)", text, re.I)
    if m:
        days = int(m.group(1))
        policy["max_age_days"] = days if days > 0 else None
    m = re.search(r"PasswordHistoryCount\s*:\s*(\d+)", text, re.I)
    if m:
        policy["history_count"] = int(m.group(1))
    m = re.search(r"LockoutThreshold\s*:\s*(\d+)", text, re.I)
    if m:
        policy["lockout_threshold"] = int(m.group(1))
    m = re.search(r"LockoutDuration\s*:\s*\.(\d+)", text, re.I)
    if m:
        policy["lockout_duration_min"] = int(m.group(1))
    m = re.search(r"ComplexityEnabled\s*:\s*(True|False)", text, re.I)
    if m:
        policy["complexity_enabled"] = m.group(1).lower() == "true"
    return policy


def parse_pam_password(text):
    policy = {}
    for line in text.splitlines():
        line = line.strip()
        if line.startswith("#") or not line:
            continue
        m = re.search(r"minlen\s*=\s*(\d+)", line)
        if m:
            policy["min_length"] = int(m.group(1))
        m = re.search(r"dcredit\s*=\s*(-?\d+)", line)
        if m:
            policy["dcredit"] = int(m.group(1))
        m = re.search(r"ucredit\s*=\s*(-?\d+)", line)
        if m:
            policy["ucredit"] = int(m.group(1))
        m = re.search(r"lcredit\s*=\s*(-?\d+)", line)
        if m:
            policy["lcredit"] = int(m.group(1))
        m = re.search(r"ocredit\s*=\s*(-?\d+)", line)
        if m:
            policy["ocredit"] = int(m.group(1))
        m = re.search(r"retry\s*=\s*(\d+)", line)
        if m:
            policy["retry"] = int(m.group(1))
    return policy


def parse_login_defs(text):
    policy = {}
    for line in text.splitlines():
        line = line.strip()
        if line.startswith("#") or not line:
            continue
        m = re.match(r"PASS_MAX_DAYS\s+(\d+)", line)
        if m:
            val = int(m.group(1))
            policy["max_age_days"] = val if val > 0 else None
        m = re.match(r"PASS_MIN_DAYS\s+(\d+)", line)
        if m:
            policy["min_age_days"] = int(m.group(1))
        m = re.match(r"PASS_MIN_LEN\s+(\d+)", line)
        if m:
            policy["min_length"] = int(m.group(1))
        m = re.match(r"PASS_WARN_AGE\s+(\d+)", line)
        if m:
            policy["warn_age_days"] = int(m.group(1))
    return policy


def parse_shadow(text):
    accounts = []
    for line in text.splitlines():
        parts = line.strip().split(":")
        if len(parts) < 2:
            continue
        username = parts[0]
        password_hash = parts[1] if len(parts) > 1 else ""
        no_password = password_hash in ("!", "*", "!!", "!", "", "x")
        expired = False
        if len(parts) > 7:
            try:
                expire_days = int(parts[7])
                if expire_days == 0:
                    pass
                else:
                    import time

                    now = time.time()
                    expire_epoch = expire_days * 86400
                    if expire_epoch < now:
                        expired = True
            except (ValueError, IndexError):
                pass
        hash_type = "unknown"
        if password_hash.startswith("$1$"):
            hash_type = "md5crypt"
        elif password_hash.startswith("$5$"):
            hash_type = "sha256crypt"
        elif password_hash.startswith("$6$"):
            hash_type = "sha512crypt"
        elif password_hash.startswith("$y$"):
            hash_type = "yescrypt"
        elif password_hash in ("!", "*", "!!"):
            hash_type = "locked/disabled"
        elif password_hash == "":
            hash_type = "empty/no_password"
        accounts.append(
            {
                "username": username,
                "hash_type": hash_type,
                "no_password": no_password,
                "expired": expired,
            }
        )
    return accounts


def parse_pwquality(text):
    policy = {}
    for line in text.splitlines():
        line = line.strip()
        if line.startswith("#") or not line or "=" not in line:
            continue
        key, _, val = line.partition("=")
        key = key.strip()
        val = val.strip().strip('"').strip("'")
        mapping = {
            "minlen": "min_length",
            "dcredit": "dcredit",
            "ucredit": "ucredit",
            "lcredit": "lcredit",
            "ocredit": "ocredit",
            "maxrepeat": "max_repeat",
            "maxclassrepeat": "max_class_repeat",
            "gecoscheck": "gecos_check",
            "dictcheck": "dict_check",
            "usercheck": "user_check",
            "enforcing": "enforcing",
        }
        if key in mapping:
            try:
                policy[mapping[key]] = int(val)
            except ValueError:
                policy[mapping[key]] = val
    return policy


def analyze_complexity(policy, results_text):
    has_complexity = False
    source = "unknown"
    if "ComplexityEnabled" in results_text:
        m = re.search(r"ComplexityEnabled\s*:\s*(True|False)", results_text, re.I)
        if m:
            has_complexity = m.group(1).lower() == "true"
            source = "ad"
    if "pwquality" in results_text.lower() or "pam_pwquality" in results_text.lower():
        pwq = parse_pwquality(results_text)
        if pwq.get("dcredit") or pwq.get("ucredit") or pwq.get("lcredit") or pwq.get("ocredit"):
            has_complexity = True
            source = "pwquality"
    min_len = policy.get("min_length", 0)
    if min_len >= 14:
        has_complexity = True
    elif min_len >= 8:
        if not has_complexity:
            has_complexity = False
    return has_complexity


def compute_risk_score(policy, has_complexity, shadow_accounts):
    score = 0
    findings = []
    min_len = policy.get("min_length", 0)
    max_age = policy.get("max_age_days")
    lockout = policy.get("lockout_threshold", 0)
    history = policy.get("history_count", 0)
    if min_len < 8:
        score += 3
        findings.append(f"Minimum password length too short ({min_len} chars, recommended >=8)")
    elif min_len < 12:
        score += 1
        findings.append(f"Minimum password length is {min_len} chars (recommended >=12)")
    else:
        score += 0
    if not has_complexity:
        score += 2
        findings.append("No password complexity requirements enforced")
    if max_age is None or max_age == 0:
        score += 2
        findings.append("Password never expires")
    elif max_age > 90:
        score += 1
        findings.append(f"Max password age is {max_age} days (recommended <=90)")
    if lockout == 0:
        score += 2
        findings.append("No account lockout threshold configured")
    elif lockout > 10:
        score += 1
        findings.append(f"Lockout threshold is {lockout} (recommended <=10)")
    if history < 3:
        score += 1
        findings.append(f"Password history only remembers {history} passwords (recommended >=5)")
    no_pass_accounts = [a for a in shadow_accounts if a["no_password"]]
    if no_pass_accounts:
        score += 2
        names = [a["username"] for a in no_pass_accounts]
        findings.append(f"Accounts with no password: {', '.join(names)}")
    expired_accounts = [a for a in shadow_accounts if a["expired"]]
    if expired_accounts:
        names = [a["username"] for a in expired_accounts]
        findings.append(f"Expired accounts still present: {', '.join(names)}")
    weak_hashes = [a for a in shadow_accounts if a["hash_type"] in ("md5crypt",)]
    if weak_hashes:
        score += 1
        names = [a["username"] for a in weak_hashes]
        findings.append(f"Accounts using weak hash (md5crypt): {', '.join(names)}")
    score = max(0, min(10, score))
    if score <= 3:
        risk = "weak"
    elif score <= 6:
        risk = "moderate"
    else:
        risk = "strong"
    return score, risk, findings


def classify_risk(score):
    if score <= 3:
        return "weak"
    elif score <= 6:
        return "moderate"
    return "strong"


def check_agent(agent, task_results):
    hostname = agent.get("hostname", "unknown")
    os_info = agent.get("os", "unknown")
    all_text = "\n".join(
        r.get("output", "") for r in task_results if isinstance(r, dict)
    )
    combined_policy = {}
    shadow_accounts = []
    for result in task_results:
        if not isinstance(result, dict):
            continue
        output = result.get("output", "")
        cmd = result.get("command", result.get("task", ""))
        if "net accounts" in cmd.lower() or "Net Accounts" in cmd or "net accounts" in output.lower():
            p = parse_net_accounts(output)
            combined_policy.update(p)
        if "Get-ADDefaultDomainPasswordPolicy" in cmd or "ADDefaultDomain" in output:
            p = parse_ad_domain_policy(output)
            combined_policy.update(p)
        if "pam" in cmd.lower() or "pam.d" in cmd.lower():
            p = parse_pam_password(output)
            combined_policy.update(p)
        if "login.defs" in cmd.lower() or "PASS_MAX_DAYS" in output:
            p = parse_login_defs(output)
            combined_policy.update(p)
        if "pwquality" in cmd.lower() or "pwquality" in output.lower():
            p = parse_pwquality(output)
            combined_policy.update(p)
        if "shadow" in cmd.lower() and ":" in output:
            shadow_accounts = parse_shadow(output)
    has_complexity = analyze_complexity(combined_policy, all_text)
    risk_score, risk_level, findings = compute_risk_score(
        combined_policy, has_complexity, shadow_accounts
    )
    if not combined_policy:
        findings.insert(0, "No password policy data found in task results")
        risk_score = 10
        risk_level = "weak"
    complexity_desc = []
    if has_complexity:
        complexity_desc.append("complexity enabled")
    if combined_policy.get("min_length", 0) >= 8:
        complexity_desc.append(f"min {combined_policy.get('min_length')} chars")
    complexity_str = "; ".join(complexity_desc) if complexity_desc else "none"
    agent_data = {
        "id": agent.get("id", "unknown"),
        "hostname": hostname,
        "os": os_info,
        "policy": {
            "min_length": combined_policy.get("min_length", 0),
            "max_age_days": combined_policy.get("max_age_days"),
            "complexity": complexity_str,
            "complexity_enabled": has_complexity,
            "lockout_threshold": combined_policy.get("lockout_threshold", 0),
            "lockout_duration_min": combined_policy.get("lockout_duration_min", 0),
            "lockout_reset_min": combined_policy.get("lockout_reset_min", 0),
            "history_count": combined_policy.get("history_count", 0),
        },
        "risk_score": risk_score,
        "risk_level": risk_level,
        "findings": findings,
        "shadow_accounts": [
            {
                "username": a["username"],
                "hash_type": a["hash_type"],
                "no_password": a["no_password"],
            }
            for a in shadow_accounts
            if a["no_password"]
        ],
    }
    return agent_data


def main():
    db = Database()
    raw = read_stdin()
    try:
        params = json.loads(raw) if raw else {}
    except json.JSONDecodeError:
        params = {}
    agent_id = params.get("agent_id")
    task_results = db.get_task_results(agent_id=agent_id) if hasattr(db, "get_task_results") else []
    agents_data = []
    if hasattr(db, "get_agents"):
        agents = db.get_agents()
        if agent_id:
            agents = [a for a in agents if a.get("id") == agent_id]
        for agent in agents:
            agent_tasks = []
            if hasattr(db, "get_task_results"):
                agent_tasks = db.get_task_results(agent_id=agent.get("id"))
            if agent_tasks:
                result = check_agent(agent, agent_tasks)
                agents_data.append(result)
    if not agents_data:
        agent_result = {
            "id": agent_id or "unknown",
            "hostname": "unknown",
            "os": "unknown",
            "policy": {
                "min_length": 0,
                "max_age_days": None,
                "complexity": "none",
                "complexity_enabled": False,
                "lockout_threshold": 0,
                "lockout_duration_min": 0,
                "lockout_reset_min": 0,
                "history_count": 0,
            },
            "risk_score": 10,
            "risk_level": "weak",
            "findings": ["No agents or task results available"],
            "shadow_accounts": [],
        }
        agents_data.append(agent_result)
    total = len(agents_data)
    weak = sum(1 for a in agents_data if a["risk_level"] == "weak")
    moderate = sum(1 for a in agents_data if a["risk_level"] == "moderate")
    strong = sum(1 for a in agents_data if a["risk_level"] == "strong")
    no_lockout = sum(
        1 for a in agents_data if a["policy"]["lockout_threshold"] == 0
    )
    no_complexity = sum(
        1 for a in agents_data if not a["policy"]["complexity_enabled"]
    )
    summary = {
        "total_agents": total,
        "weak_policy": weak,
        "moderate_policy": moderate,
        "strong_policy": strong,
        "no_lockout": no_lockout,
        "no_complexity": no_complexity,
    }
    data = {"agents": agents_data, "summary": summary}
    output = f"Checked {total} agents | {weak} weak policies | {no_lockout} no lockout | {no_complexity} no complexity"
    write_result(True, output=output, data=data)


if __name__ == "__main__":
    main()
