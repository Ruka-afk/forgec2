#!/usr/bin/env python3
"""Network Enumerator plugin — extracts listening ports, connections, and DNS info from agent task results."""

import json
import os
import re
import sys
from collections import Counter

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

NETSTAT_IPV4_RE = re.compile(
    r"^(TCP|UDP)\s+"
    r"(\S+:\d+)\s+"
    r"(\S+:\d+)?\s*"
    r"(LISTENING|ESTABLISHED|CLOSE_WAIT|TIME_WAIT|SYN_SENT|SYN_RECEIVED|FIN_WAIT|CLOSING|UNKNOWN)?",
    re.MULTILINE | re.IGNORECASE,
)

LINUX_NETSTAT_RE = re.compile(
    r"^(tcp|tcp6|udp|udp6)\s+\S+\s+\S+\s+(\S+:\d+)\s+(\S+)?\s+(LISTEN|ESTABLISHED|TIME_WAIT|CLOSE_WAIT|SYN_SENT)?",
    re.MULTILINE | re.IGNORECASE,
)

SS_RE = re.compile(
    r"^(tcp|udp|tcp6|udp6)\s+\S+\s+\S+\s+(\S+:\d+)\s+(\S+)?\s+(LISTEN|ESTABLISHED|TIME_WAIT|CLOSE_WAIT|SYN_SENT)?",
    re.MULTILINE | re.IGNORECASE,
)

IPCONFIG_DNS_RE = re.compile(r"(?:DNS Servers|dns-server)\s*[:=]\s*(\S+)", re.IGNORECASE)
IPCONFIG_GW_RE = re.compile(r"(?:Default Gateway|gateway)\s*[:=]\s*(\S+)", re.IGNORECASE)
IPCONFIG_SUBNET_RE = re.compile(r"(?:Subnet Mask|prefix)\s*[:=]\s*(\S+)", re.IGNORECASE)

LINUX_DNS_RE = re.compile(r"^nameserver\s+(\S+)", re.MULTILINE)
LINUX_GW_RE = re.compile(r"default via\s+(\S+)")
LINUX_IFACE_RE = re.compile(r"inet\s+(\S+/\d+)")


def _parse_windows_netstat(text):
    listening = []
    connections = []
    for m in NETSTAT_IPV4_RE.finditer(text):
        proto = m.group(1).upper()
        local = m.group(2)
        remote = m.group(3) or ""
        state = (m.group(4) or "").upper()

        local_parts = local.rsplit(":", 1)
        local_ip = local_parts[0] if len(local_parts) > 1 else local
        local_port = int(local_parts[1]) if len(local_parts) > 1 else 0

        entry = {
            "port": local_port,
            "protocol": proto,
            "local": local,
            "ip": local_ip,
        }

        if state == "LISTENING":
            listening.append(entry)
        else:
            remote_parts = remote.rsplit(":", 1)
            connections.append({
                "local": local,
                "remote": remote,
                "remote_ip": remote_parts[0] if len(remote_parts) > 1 else remote,
                "remote_port": int(remote_parts[1]) if len(remote_parts) > 1 else 0,
                "state": state,
                "protocol": proto,
            })
    return listening, connections


def _parse_linux_netstat(text):
    listening = []
    connections = []
    for m in LINUX_NETSTAT_RE.finditer(text):
        proto = m.group(1).upper().replace("6", "")
        local = m.group(2)
        remote = m.group(3) or ""
        state = (m.group(4) or "").upper()

        local_parts = local.rsplit(":", 1)
        local_port = int(local_parts[1]) if len(local_parts) > 1 else 0

        entry = {
            "port": local_port,
            "protocol": proto,
            "local": local,
            "ip": local_parts[0] if len(local_parts) > 1 else local,
        }

        if state == "LISTEN":
            listening.append(entry)
        elif remote:
            remote_parts = remote.rsplit(":", 1)
            connections.append({
                "local": local,
                "remote": remote,
                "remote_ip": remote_parts[0] if len(remote_parts) > 1 else remote,
                "remote_port": int(remote_parts[1]) if len(remote_parts) > 1 else 0,
                "state": state,
                "protocol": proto,
            })
    return listening, connections


def _parse_ss(text):
    listening = []
    connections = []
    for m in SS_RE.finditer(text):
        proto = m.group(1).upper().replace("6", "")
        local = m.group(2)
        remote = m.group(3) or ""
        state = (m.group(4) or "").upper()

        local_parts = local.rsplit(":", 1)
        local_port = int(local_parts[1]) if len(local_parts) > 1 else 0

        entry = {
            "port": local_port,
            "protocol": proto,
            "local": local,
            "ip": local_parts[0] if len(local_parts) > 1 else local,
        }

        if state == "LISTEN":
            listening.append(entry)
        elif remote and remote != "*:*":
            remote_parts = remote.rsplit(":", 1)
            connections.append({
                "local": local,
                "remote": remote,
                "remote_ip": remote_parts[0] if len(remote_parts) > 1 else remote,
                "remote_port": int(remote_parts[1]) if len(remote_parts) > 1 else 0,
                "state": state,
                "protocol": proto,
            })
    return listening, connections


def _parse_dns_config(text):
    dns_servers = []
    gateway = ""
    subnet = ""

    for m in IPCONFIG_DNS_RE.finditer(text):
        ip = m.group(1)
        if re.match(r"\d+\.\d+\.\d+\.\d+", ip):
            dns_servers.append(ip)
    for m in LINUX_DNS_RE.finditer(text):
        dns_servers.append(m.group(1))

    gw = IPCONFIG_GW_RE.search(text) or LINUX_GW_RE.search(text)
    if gw:
        gateway = gw.group(1)

    sub = IPCONFIG_SUBNET_RE.search(text)
    if sub:
        subnet = sub.group(1)
    iface = LINUX_IFACE_RE.search(text)
    if iface and not subnet:
        subnet = iface.group(1)

    return dns_servers, gateway, subnet


def _has_network_command(cmd, result):
    combined = (cmd + " " + result).lower()
    keywords = [
        "netstat", "ss -", "ss -t", "ss -u", "ipconfig", "ifconfig",
        "ip addr", "ip -", "network", "listening", "established",
    ]
    return any(kw in combined for kw in keywords)


def main():
    data = read_stdin()
    params = data.get("params", {})
    target_agent = params.get("agent_id") or ""
    ports_only = params.get("ports_only", False)

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        agents = db.all_agents()
        network_hosts = db.all_network_hosts()

        if target_agent:
            agents = [a for a in agents if a["id"] == target_agent]

        # Index network hosts by agent_id
        hosts_by_agent = {}
        for h in network_hosts:
            aid = h.get("agent_id", "")
            hosts_by_agent.setdefault(aid, []).append(h)

        agent_data = []
        all_listen_ports = []
        all_connections = []
        all_remote_ips = Counter()
        port_counter = Counter()

        for agent in agents:
            aid = agent["id"]
            tasks = db.tasks_for_agent(aid)

            listening = []
            connections = []
            dns_servers = []
            gateway = ""
            subnet = ""

            for task in tasks:
                if task.get("status") != "completed":
                    continue
                cmd = task.get("command", "")
                result = task.get("result", "")
                if not result or not _has_network_command(cmd, result):
                    continue

                # Detect platform from agent OS
                agent_os = (agent.get("os") or "").lower()
                is_linux = any(x in agent_os for x in ["linux", "unix", "darwin", "macos"])

                if "netstat" in cmd.lower() or "netstat" in result[:200].lower():
                    if is_linux:
                        l, c = _parse_linux_netstat(result)
                    else:
                        l, c = _parse_windows_netstat(result)
                    listening.extend(l)
                    connections.extend(c)
                elif "ss " in cmd.lower():
                    l, c = _parse_ss(result)
                    listening.extend(l)
                    connections.extend(c)
                elif "ipconfig" in cmd.lower() or "ifconfig" in cmd.lower() or "ip addr" in cmd.lower():
                    l, c = _parse_linux_netstat(result) if is_linux else _parse_windows_netstat(result)
                    listening.extend(l)
                    connections.extend(c)

                d, g, s = _parse_dns_config(result)
                if d:
                    dns_servers.extend(d)
                if g and not gateway:
                    gateway = g
                if s and not subnet:
                    subnet = s

            # Deduplicate
            seen_ports = set()
            unique_listening = []
            for p in listening:
                key = (p["port"], p["protocol"])
                if key not in seen_ports:
                    seen_ports.add(key)
                    unique_listening.append(p)

            seen_conns = set()
            unique_connections = []
            for c in connections:
                key = (c["local"], c["remote"], c["state"])
                if key not in seen_conns:
                    seen_conns.add(key)
                    unique_connections.append(c)

            # Include network hosts as additional connections
            extra_hosts = hosts_by_agent.get(aid, [])
            for h in extra_hosts:
                hsvc = h.get("services", "")
                hports = []
                try:
                    hports = json.loads(hsvc) if hsvc else []
                except (json.JSONDecodeError, TypeError):
                    pass
                for hp in hports:
                    if isinstance(hp, dict):
                        port = hp.get("port", 0)
                        proto = hp.get("protocol", "tcp").upper()
                    else:
                        port = int(hp) if str(hp).isdigit() else 0
                        proto = "TCP"
                    entry = {"port": port, "protocol": proto, "local": "", "ip": h.get("ip", "")}
                    k = (entry["port"], entry["protocol"])
                    if k not in seen_ports:
                        seen_ports.add(k)
                        unique_listening.append(entry)

            agent_data.append({
                "id": aid,
                "hostname": agent.get("hostname") or aid[:8],
                "listening_ports": unique_listening,
                "connections": unique_connections,
                "dns_servers": list(set(dns_servers)),
                "gateway": gateway,
                "subnet": subnet,
                "discovered_hosts": len(extra_hosts),
            })

            all_listen_ports.extend(unique_listening)
            all_connections.extend(unique_connections)
            for c in unique_connections:
                rip = c.get("remote_ip", "")
                if rip:
                    all_remote_ips[rip] += 1

        # Build summary
        for p in all_listen_ports:
            port_counter[p["port"]] += 1

        common_ports = dict(port_counter.most_common(20))

        summary = {
            "total_agents": len(agent_data),
            "total_listening_ports": len(all_listen_ports),
            "unique_remote_ips": len(all_remote_ips),
            "total_established": len(all_connections),
            "common_ports": common_ports,
            "top_remote_ips": dict(all_remote_ips.most_common(10)),
        }

        output = (
            f"Enumerated {len(agent_data)} agents | "
            f"{len(all_listen_ports)} listening ports | "
            f"{len(all_connections)} established connections"
        )

        write_result(
            True,
            output=output,
            data={
                "agents": agent_data,
                "summary": summary,
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
