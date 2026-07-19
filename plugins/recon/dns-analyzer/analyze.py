#!/usr/bin/env python3
"""DNS Analyzer plugin — parses DNS cache, queries, resolver config, and detects anomalies from agent task results."""

import math
import os
import re
import sys
from collections import Counter, defaultdict

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

# Windows: ipconfig /displaydns output block parser
# Each record block starts with "    Record Name . . . . . : " and contains Type / Data
WIN_DNS_RECORD_RE = re.compile(
    r"Record Name\s*[:.]+\s*(\S+).*?"
    r"Record Type\s*[:.]+\s*(\d+).*?"
    r"(?:A \(Host\)|AAAA \(Host\)|PTR \(Pointer\)|CNAME \(Alias\)|MX \(Mail Exchanger\)|"
    r"TXT \(Text\)|SRV \(Service\)|SOA \(Start of Authority\))\s*[:.]+\s*(.+?)(?=\n\n|\n\s{2,}Record Name|\Z)",
    re.DOTALL | re.IGNORECASE,
)
WIN_DNS_IP_RE = re.compile(r"\b(\d{1,3}(?:\.\d{1,3}){3})\b")

# PowerShell Get-DnsClientCache output
PS_DNS_CACHE_RE = re.compile(
    r"(?:Entry|Name)\s*[:=]\s*(\S+).*?(?:Type|RecordType)\s*[:=]\s*(\S+).*?(?:Data|DataLength)\s*[:=]\s*(.+?)(?=\n[A-Z]|\Z)",
    re.DOTALL | re.IGNORECASE,
)

# nslookup output
NSLOOKUP_HEADER_RE = re.compile(
    r"Name:\s*(\S+)\s*\n\s*Address(?:es)?:\s*(.+)", re.IGNORECASE
)
NSLOOKUP_SERVER_RE = re.compile(r"Default Server:\s*(.+)", re.IGNORECASE)

# resolv.conf
RESOLV_NAMESERVER_RE = re.compile(r"^nameserver\s+(\S+)", re.MULTILINE)

# dig +short
DIG_SHORT_RE = re.compile(r"^([^\n]+)$", re.MULTILINE)

# host command
HOST_RESULT_RE = re.compile(r"(\S+)\s+has address\s+(\S+)", re.IGNORECASE)

# CNAME chain from dig / nslookup / host
CNAME_RE = re.compile(
    r"(\S+)\s+(?:is an alias for|CNAME)\s+(\S+)", re.IGNORECASE
)

# TXT record lines
TXT_RE = re.compile(
    r"(?:text|txt)\s*[:=]\s*[\"']?(.+?)[\"']?\s*$", re.MULTILINE | re.IGNORECASE
)

# DNS-over-HTTPS indicators
DOH_RE = re.compile(r"dns\.google|cloudflare-dns|doh\.|dns-over-https|1\.1\.1\.1|8\.8\.8\.8", re.IGNORECASE)

RECORD_TYPE_MAP = {
    "1": "A", "2": "NS", "5": "CNAME", "6": "SOA", "12": "PTR",
    "15": "MX", "16": "TXT", "28": "AAAA", "33": "SRV", "43": "DS",
    "257": "CAA",
}

CLOUD_DOMAINS = {
    "amazonaws.com", "aws", "azure", "windows.net", "windows.com",
    "googleapis.com", "google.com", "gstatic.com", "googleusercontent.com",
    "cloudflare.com", "cloudfront.net", "akamai", "akamaized.net", "akamaihd.net",
    "digitaloceanspaces.com", "herokuapp.com", "heroku.com",
    "blob.core.windows.net", "s3.amazonaws.com", "s3",
    "ghcr.io", "docker.io", "docker.com",
    "office365.com", "microsoft.com", "outlook.com", "live.com",
    "office.com", "azureedge.net", "azure.com",
}

CDN_DOMAINS = {
    "cloudfront.net", "cloudflare.com", "akamai", "akamaized.net", "akamaihd.net",
    "fastly.net", "fastly.com", "edgekey.net", "edgesuite.net",
    "cdn77.org", "cdn77.com", "keycdn.com", "stackpath.com",
    "incapsula.com", "imperva.com", "sucuri.net", "akamaized.net",
}

C2_LIKE_PATTERNS = [
    r"\.xyz$", r"\.top$", r"\.buzz$", r"\.icu$", r"\.tk$", r"\.ml$",
    r"\.ga$", r"\.cf$", r"\.gq$", r"\.cc$", r"\.pw$", r"\.cc$",
    r"[a-z0-9]{20,}\.", r"(?:update|config|check|status|data|cdn)\d*\.",
]

SUSPICIOUS_TLD = {
    "xyz", "top", "buzz", "icu", "tk", "ml", "ga", "cf", "gq", "pw",
    "work", "click", "link", "loan", "racing", "win", "bid",
}


def _entropy(s):
    if not s:
        return 0.0
    freq = Counter(s)
    length = len(s)
    return -sum((c / length) * math.log2(c / length) for c in freq.values())


def _categorize_domain(domain):
    dl = domain.lower().rstrip(".")
    for cloud in CLOUD_DOMAINS:
        if cloud in dl or dl.endswith("." + cloud):
            return "cloud"
    for cdn in CDN_DOMAINS:
        if cdn in dl or dl.endswith("." + cdn):
            return "cdn"
    for pat in C2_LIKE_PATTERNS:
        if re.search(pat, dl):
            return "suspicious"
    tld = dl.rsplit(".", 1)[-1] if "." in dl else ""
    if tld in SUSPICIOUS_TLD:
        return "suspicious"
    parts = dl.split(".")
    if len(parts) >= 4:
        return "suspicious"
    return "external"


def _is_dga_candidate(domain):
    parts = domain.lower().rstrip(".").split(".")
    for part in parts:
        if len(part) > 12 and _entropy(part) > 3.5:
            return True
        if len(part) > 8 and not any(c in part for c in ".-") and _entropy(part) > 3.8:
            return True
    if len(parts) >= 5:
        labels = [len(p) for p in parts if p]
        avg = sum(labels) / len(labels) if labels else 0
        if avg > 5 and _entropy("".join(parts)) > 3.2:
            return True
    return False


def _has_dns_tunnel_indicators(domain, record_type, data=""):
    flags = []
    rt = record_type.upper()
    if rt in ("TXT", "NULL", "CNAME", "MX"):
        flags.append(f"unusual_type:{rt}")
    if len(data) > 200:
        flags.append("large_record")
    if re.match(r"^[a-zA-Z0-9+/=]{40,}$", data.replace(" ", "")):
        flags.append("base64_payload")
    parts = domain.lower().split(".")
    for p in parts:
        if len(p) > 20 and _entropy(p) > 3.8:
            flags.append("high_entropy_label")
            break
    return flags


def _parse_windows_dns_cache(text):
    records = []
    blocks = re.split(r"(?=Record Name)", text)
    for block in blocks:
        name_m = re.search(r"Record Name\s*[:.]+\s*(\S+)", block, re.IGNORECASE)
        type_m = re.search(r"Record Type\s*[:.]+\s*(\d+)", block, re.IGNORECASE)
        data_m = re.search(r"(?:Host Record|Data)\s*[:.]+\s*(.+?)(?:\n\s{2,}|\Z)", block, re.DOTALL | re.IGNORECASE)
        if name_m and type_m:
            rt = RECORD_TYPE_MAP.get(type_m.group(1), type_m.group(1))
            data = data_m.group(1).strip() if data_m else ""
            ip = ""
            ip_m = WIN_DNS_IP_RE.search(data)
            if ip_m:
                ip = ip_m.group(1)
            records.append({
                "name": name_m.group(1).rstrip("."),
                "type": rt,
                "data": data,
                "ip": ip,
            })
    return records


def _parse_ps_dns_cache(text):
    records = []
    blocks = re.split(r"\n(?=(?:Entry|Name)\s*[:=])", text)
    for block in blocks:
        name_m = re.search(r"(?:Entry|Name)\s*[:=]\s*(\S+)", block, re.IGNORECASE)
        type_m = re.search(r"(?:Type|RecordType)\s*[:=]\s*(\S+)", block, re.IGNORECASE)
        data_m = re.search(r"(?:Data)\s*[:=]\s*(.+?)(?:\n|\Z)", block, re.IGNORECASE)
        if name_m:
            records.append({
                "name": name_m.group(1).rstrip("."),
                "type": (type_m.group(1) if type_m else "Unknown").upper(),
                "data": data_m.group(1).strip() if data_m else "",
            })
    return records


def _parse_nslookup(text):
    results = []
    for m in NSLOOKUP_HEADER_RE.finditer(text):
        domain = m.group(1).rstrip(".")
        addrs = [a.strip() for a in m.group(2).replace(",", " ").split() if a.strip()]
        for addr in addrs:
            results.append({"domain": domain, "ip": addr, "type": "A"})
    return results


def _parse_dig_short(text, full_output=""):
    results = []
    for m in DIG_SHORT_RE.finditer(text):
        line = m.group(1).strip()
        if not line or line.startswith(";") or line.startswith("#"):
            continue
        parts = line.split()
        if len(parts) == 1:
            ip_m = WIN_DNS_IP_RE.match(parts[0])
            if ip_m:
                results.append({"data": parts[0], "type": "A"})
            elif not parts[0].startswith(";"):
                results.append({"data": parts[0], "type": "CNAME"})
        elif len(parts) >= 2:
            results.append({"data": parts[-1], "type": parts[-2].upper() if parts[-2].upper() in RECORD_TYPE_MAP.values() else "A"})
    return results


def _parse_host_command(text):
    results = []
    for m in HOST_RESULT_RE.finditer(text):
        results.append({"domain": m.group(1), "ip": m.group(2), "type": "A"})
    return results


def _extract_cnames(text):
    chains = []
    for m in CNAME_RE.finditer(text):
        chains.append({"from": m.group(1), "to": m.group(2)})
    return chains


def _extract_txt_records(text):
    return [m.group(1).strip() for m in TXT_RE.finditer(text)]


def _is_dns_command(cmd, result):
    combined = (cmd + " " + result).lower()
    keywords = [
        "displaydns", "get-dnsclientcache", "nslookup", "dig ",
        "resolv.conf", "/etc/resolv", "host ", "dnsmasq",
        "dns-server", "nameserver", "ipconfig /all", "dns",
    ]
    return any(kw in combined for kw in keywords)


def main():
    data = read_stdin()
    params = data.get("params", {})
    target_agent = params.get("agent_id") or ""

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        agents = db.all_agents()
        if target_agent:
            agents = [a for a in agents if a["id"] == target_agent]

        all_domain_counts = Counter()
        all_categories = Counter()
        all_dga_candidates = []
        all_dns_tunnel_candidates = []
        all_cname_chains = []
        all_txt_records = []
        agent_results = []

        for agent in agents:
            aid = agent["id"]
            hostname = agent.get("hostname") or aid[:8]
            tasks = db.tasks_for_agent(aid)

            dns_servers = set()
            domain_records = defaultdict(lambda: {"count": 0, "type": "", "category": ""})
            suspicious_domains = []
            cnames = []
            txt_records = []
            cache_records = []

            for task in tasks:
                if task.get("status") != "completed":
                    continue
                cmd = task.get("command", "")
                result = task.get("result", "")
                if not result or not _is_dns_command(cmd, result):
                    continue

                # Parse DNS servers from resolv.conf
                for m in RESOLV_NAMESERVER_RE.finditer(result):
                    dns_servers.add(m.group(1))

                # Parse DNS servers from nslookup header
                for m in NSLOOKUP_SERVER_RE.finditer(result):
                    srv = m.group(1).strip()
                    ip_m = WIN_DNS_IP_RE.search(srv)
                    if ip_m:
                        dns_servers.add(ip_m.group(1))

                # Windows DNS cache
                if "displaydns" in cmd.lower():
                    cache_records.extend(_parse_windows_dns_cache(result))

                # PowerShell DNS cache
                if "get-dnsclientcache" in cmd.lower():
                    cache_records.extend(_parse_ps_dns_cache(result))

                # nslookup results
                nslookup = _parse_nslookup(result)
                for r in nslookup:
                    dom = r["domain"]
                    domain_records[dom]["count"] += 1
                    domain_records[dom]["type"] = r["type"]

                # dig / host
                dig = _parse_dig_short(result, result)
                for r in dig:
                    dom = r.get("domain", "")
                    data_val = r.get("data", "")
                    if data_val and WIN_DNS_IP_RE.match(data_val):
                        dom = dom or data_val
                    if dom:
                        domain_records[dom]["count"] += 1
                        domain_records[dom]["type"] = r.get("type", "A")

                host_res = _parse_host_command(result)
                for r in host_res:
                    domain_records[r["domain"]]["count"] += 1
                    domain_records[r["domain"]]["type"] = r["type"]

                # CNAME chains
                cnames.extend(_extract_cnames(result))

                # TXT records
                txt_records.extend(_extract_txt_records(result))

            # Categorize all domains
            queried_domains = []
            for dom, info in domain_records.items():
                cat = _categorize_domain(dom)
                info["category"] = cat
                all_domain_counts[dom] += info["count"]
                all_categories[cat] += 1
                queried_domains.append({
                    "domain": dom,
                    "type": info["type"],
                    "category": cat,
                    "count": info["count"],
                })
                if cat == "suspicious":
                    suspicious_domains.append(dom)

            # DGA check
            agent_dga = []
            for dom in domain_records:
                if _is_dga_candidate(dom):
                    agent_dga.append(dom)
                    all_dga_candidates.append(dom)

            # DNS tunnel check
            agent_tunnel = []
            for rec in cache_records:
                flags = _has_dns_tunnel_indicators(rec.get("name", ""), rec.get("type", ""), rec.get("data", ""))
                if flags:
                    entry = {"domain": rec["name"], "type": rec["type"], "flags": flags}
                    agent_tunnel.append(entry)
                    all_dns_tunnel_candidates.append(entry)

            for cname in cnames:
                if cname not in all_cname_chains:
                    all_cname_chains.append(cname)

            all_txt_records.extend(txt_records)

            agent_results.append({
                "id": aid,
                "hostname": hostname,
                "dns_servers": sorted(dns_servers),
                "queried_domains": sorted(queried_domains, key=lambda d: d["count"], reverse=True),
                "suspicious_domains": sorted(set(suspicious_domains)),
                "cname_chains": cnames,
                "txt_records": txt_records,
                "dga_candidates": agent_dga,
                "tunnel_candidates": agent_tunnel,
                "cache_record_count": len(cache_records),
            })

        # Build summary
        unique_domains = len(all_domain_counts)
        total_suspicious = len(set(all_dga_candidates))

        by_cat = Counter()
        for ar in agent_results:
            for qd in ar["queried_domains"]:
                by_cat[qd["category"]] += 1

        summary = {
            "total_agents": len(agent_results),
            "unique_domains": unique_domains,
            "by_category": dict(by_cat),
            "dga_candidates": sorted(set(all_dga_candidates))[:20],
            "dns_tunnel_candidates": all_dns_tunnel_candidates[:20],
            "cname_chains": all_cname_chains[:30],
            "txt_record_count": len(all_txt_records),
        }

        output = (
            f"Analyzed {len(agent_results)} agents | "
            f"{unique_domains} unique domains | "
            f"{len(set(all_dga_candidates))} suspicious | "
            f"{len(all_dns_tunnel_candidates)} DGA candidates"
        )

        write_result(
            True,
            output=output,
            data={
                "agents": agent_results,
                "summary": summary,
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
