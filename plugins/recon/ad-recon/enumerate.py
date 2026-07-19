#!/usr/bin/env python3
"""AD Recon plugin — Active Directory enumeration from agent task results."""

import sys
import os
import re

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result


def _extract_domain_name(lines):
    for line in lines:
        m = re.search(r"USERDNSDOMAIN=(\S+)", line, re.IGNORECASE)
        if m:
            return m.group(1).strip('"').rstrip("\\")
    return None


def _extract_domain_controllers(lines):
    dcs = set()
    for line in lines:
        m = re.search(r"\.(?:dc|domain)\s+\S+\s+PDC\s+(\S+)", line)
        if m:
            dcs.add(m.group(1))
            continue
        m = re.search(r"DC:\s+(\S+)", line)
        if m:
            dcs.add(m.group(1))
            continue
        m = re.search(r"(\S+\.(?:dc|domain|ad)\b)", line)
        if m and "." in m.group(1) and len(m.group(1)) > 4:
            dcs.add(m.group(1))
    return sorted(dcs)


def _extract_domain_users(lines):
    users = []
    collecting = False
    for line in lines:
        stripped = line.strip()
        if re.match(r"^User accounts? for", stripped, re.IGNORECASE):
            collecting = True
            continue
        if collecting:
            if stripped == "" or re.match(r"^The command", stripped, re.IGNORECASE):
                collecting = False
                continue
            if re.match(r"^-{3,}$", stripped):
                continue
            parts = stripped.split()
            if parts and not parts[0].startswith("---"):
                users.append(parts[0])
    return users


def _extract_domain_groups(lines):
    groups = []
    collecting = False
    for line in lines:
        stripped = line.strip()
        if re.match(r"^Group accounts? for", stripped, re.IGNORECASE):
            collecting = True
            continue
        if collecting:
            if stripped == "" or re.match(r"^The command", stripped, re.IGNORECASE):
                collecting = False
                continue
            if re.match(r"^-{3,}$", stripped):
                continue
            parts = stripped.split()
            if parts and not parts[0].startswith("---"):
                groups.append(parts[0])
    return groups


def _extract_privileged_groups(lines):
    priv = {}
    current_group = None
    members = []
    for line in lines:
        stripped = line.strip()
        m = re.match(r"^Group name:\s+(.+)", stripped, re.IGNORECASE)
        if m:
            if current_group and members:
                priv[current_group] = members
            current_group = m.group(1).strip()
            members = []
            continue
        if current_group:
            if re.match(r"^-{3,}$", stripped):
                continue
            if stripped == "" or re.match(r"^The command", stripped, re.IGNORECASE):
                if members:
                    priv[current_group] = members
                current_group = None
                members = []
                continue
            if re.match(r"^Members$", stripped, re.IGNORECASE):
                continue
            if stripped and not stripped.startswith("---"):
                members.append(stripped)
    if current_group and members:
        priv[current_group] = members
    return priv


def _extract_spns(lines):
    spns = []
    for line in lines:
        m = re.search(r"(\S+/\S+@\S+)", line)
        if m:
            spns.append(m.group(1))
            continue
        m = re.search(r"^(.+)\s+(\S+/\S+)$", line)
        if m:
            spns.append(m.group(2))
    return spns


def _extract_sid(lines):
    for line in lines:
        m = re.search(r"(S-1-5-21-\d+-\d+-\d+-\d+)", line)
        if m:
            return m.group(1)
    return None


def _extract_trusts(lines):
    trusts = []
    for line in lines:
        m = re.search(
            r"(\S+)\s+(Primary|Trusting|Trusted)\s+(\S+)", line, re.IGNORECASE
        )
        if m:
            trusts.append(
                {"domain": m.group(1), "type": m.group(2), "direction": m.group(3)}
            )
            continue
        m = re.search(r"Domain:\s+(\S+)", line, re.IGNORECASE)
        if m:
            trusts.append({"domain": m.group(1), "type": "external", "direction": ""})
    return trusts


def _extract_gpos(lines):
    gpos = []
    for line in lines:
        m = re.search(r"\{([A-F0-9-]+)\}\s+(.+)", line, re.IGNORECASE)
        if m:
            gpos.append({"id": m.group(1), "name": m.group(2).strip()})
            continue
        m = re.search(r"GPO:\s+(.+)", line, re.IGNORECASE)
        if m:
            gpos.append({"id": "", "name": m.group(1).strip()})
    return gpos


def _extract_groups_from_whoami(lines):
    groups = []
    for line in lines:
        m = re.search(r"(S-1-5-21-\S+)\s+\(([^)]+)\)", line)
        if m:
            groups.append({"sid": m.group(1), "name": m.group(2)})
    return groups


def _scan_task_output(task):
    output = task.get("output", "") or ""
    if not output:
        result_data = task.get("result", "") or ""
        if isinstance(result_data, str):
            output = result_data
    raw = output if isinstance(output, str) else str(output)
    return raw


def _analyze_agent(db, agent):
    agent_id = agent.get("id", "")
    hostname = agent.get("hostname", "")
    domain = agent.get("domain", "") or ""
    result = {
        "id": agent_id,
        "hostname": hostname,
        "domain": domain,
        "domain_controllers": [],
        "users": [],
        "privileged_groups": {},
        "spns": [],
        "trusts": [],
        "gpos": [],
        "sid": None,
        "groups": [],
    }

    tasks = db.tasks_for_agent(agent_id)
    all_domain_users = set()
    all_groups = set()
    all_spns = set()
    all_dcs = set()
    all_trusts = []
    all_gpos = []
    found_domain = domain
    found_sid = None
    all_priv = {}
    all_whoami_groups = []

    for task in tasks:
        raw = _scan_task_output(task)
        if not raw:
            continue
        lines = raw.splitlines()

        task_type = (task.get("type", "") or "").lower()
        cmd_lower = (task.get("command", "") or "").lower()

        if "userdnsdomain" in raw.lower():
            d = _extract_domain_name(lines)
            if d:
                found_domain = d

        if "nltest" in cmd_lower or "dclist" in cmd_lower or "domain controllers" in raw.lower():
            for dc in _extract_domain_controllers(lines):
                all_dcs.add(dc)

        if "net" in cmd_lower and "user" in cmd_lower and "domain" in cmd_lower:
            for u in _extract_domain_users(lines):
                all_domain_users.add(u)

        if "net" in cmd_lower and "group" in cmd_lower and "domain" in cmd_lower:
            for g in _extract_domain_groups(lines):
                all_groups.add(g)

        if "domain admins" in cmd_lower or "enterprise admins" in cmd_lower:
            all_priv.update(_extract_privileged_groups(lines))

        if "set" in cmd_lower and "userdnsdomain" in cmd_lower:
            d = _extract_domain_name(lines)
            if d:
                found_domain = d

        if "whoami" in cmd_lower and "all" in cmd_lower:
            sid = _extract_sid(lines)
            if sid:
                found_sid = sid
            all_whoami_groups.extend(_extract_groups_from_whoami(lines))

        if "spn" in raw.lower() or "kerberoast" in raw.lower():
            for spn in _extract_spns(lines):
                all_spns.add(spn)

        if "trust" in raw.lower() or "nltest" in cmd_lower:
            all_trusts.extend(_extract_trusts(lines))

        if "gpo" in raw.lower() or "group policy" in raw.lower():
            all_gpos.extend(_extract_gpos(lines))

        for spn in _extract_spns(lines):
            if "/" in spn and "@" in spn:
                all_spns.add(spn)

    result["domain"] = found_domain
    result["domain_controllers"] = sorted(all_dcs)
    result["users"] = sorted(all_domain_users)
    result["privileged_groups"] = all_priv
    result["spns"] = sorted(all_spns)
    result["trusts"] = all_trusts
    result["gpos"] = all_gpos
    result["sid"] = found_sid
    result["groups"] = sorted(set(g.get("name", "") for g in all_whoami_groups if g.get("name")))

    return result


def main():
    data = read_stdin()
    params = data.get("params", {})
    target_agent = params.get("agent_id", "") or ""

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        if target_agent:
            agent = db.agent_by_id(target_agent)
            if not agent:
                write_result(False, error=f"Agent {target_agent} not found")
                return
            agents = [agent]
        else:
            agents = db.all_agents()

        if not agents:
            write_result(True, output="No agents found", data={"agents": [], "summary": {}})
            return

        analyzed = []
        for agent in agents:
            analyzed.append(_analyze_agent(db, agent))

        domain_joined = [a for a in analyzed if a["domain"]]
        unique_domains = sorted(set(a["domain"] for a in domain_joined if a["domain"]))
        all_users = set()
        all_priv_users = set()
        all_spns = set()
        all_dcs = set()

        for a in analyzed:
            all_users.update(a["users"])
            all_spns.update(a["spns"])
            all_dcs.update(a["domain_controllers"])
            for members in a["privileged_groups"].values():
                all_priv_users.update(members)

        summary = {
            "total_agents": len(analyzed),
            "domain_joined": len(domain_joined),
            "unique_domains": unique_domains,
            "total_users": len(all_users),
            "privileged_users": len(all_priv_users),
            "spn_count": len(all_spns),
            "dc_count": len(all_dcs),
        }

        output = (
            f"Enumerated {len(domain_joined)} domain-joined agents | "
            f"{len(all_users)} users | "
            f"{len(all_priv_users)} privileged | "
            f"{len(all_spns)} SPNs"
        )

        write_result(True, output=output, data={"agents": analyzed, "summary": summary})
    finally:
        db.close()


if __name__ == "__main__":
    main()
