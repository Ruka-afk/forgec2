#!/usr/bin/env python3
import sys, os, re
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

WIFI_PROFILE_RE = re.compile(r"All User Profile\s*:\s*(.+)", re.IGNORECASE)
SSID_RE = re.compile(r"SSID name\s*:\s*\"?(.+?)\"?\s*$", re.IGNORECASE)
AUTH_RE = re.compile(r"Authentication\s*:\s*(.+)", re.IGNORECASE)
ENCR_RE = re.compile(r"Encryption\s*:\s*(.+)", re.IGNORECASE)
KEY_RE = re.compile(r"Key Content\s*:\s*(.+)", re.IGNORECASE)
WPA_RE = re.compile(r"(WPA2|WPA3|WPA|OWE|SAE|PSK|Enterprise|Open|802\.1[1xX])", re.IGNORECASE)


def parse_netsh_profiles(text):
    profiles = []
    current = {}
    for line in text.splitlines():
        line = line.strip()
        m = SSID_RE.match(line)
        if m:
            if current.get("ssid"):
                profiles.append(current)
            current = {"ssid": m.group(1).strip('"'), "auth": "", "encryption": "", "password": ""}
            continue
        m = AUTH_RE.match(line)
        if m and current is not None:
            current["auth"] = m.group(1).strip()
            continue
        m = ENCR_RE.match(line)
        if m and current is not None:
            current["encryption"] = m.group(1).strip()
            continue
        m = KEY_RE.match(line)
        if m and current is not None:
            current["password"] = m.group(1).strip()
    if current.get("ssid"):
        profiles.append(current)
    return profiles


def parse_wpa_supplicant(text):
    profiles = []
    current = {}
    for line in text.splitlines():
        line = line.strip()
        if line.startswith("[") and line.endswith("]"):
            if current.get("ssid"):
                profiles.append(current)
            current = {"ssid": "", "auth": "", "encryption": "", "password": ""}
            continue
        if line.lower().startswith("ssid="):
            current["ssid"] = line.split("=", 1)[1].strip('"')
        elif line.lower().startswith("psk="):
            current["password"] = line.split("=", 1)[1].strip('"')
            current["auth"] = "WPA-PSK"
        elif line.lower().startswith("key_mgmt="):
            val = line.split("=", 1)[1].strip()
            if val == "NONE":
                current["auth"] = "Open"
            else:
                current["auth"] = val
        elif line.lower().startswith("proto="):
            current["encryption"] = line.split("=", 1)[1].strip()
    if current.get("ssid"):
        profiles.append(current)
    return profiles


def parse_nm_connections(text):
    profiles = []
    current = {}
    for line in text.splitlines():
        line = line.strip()
        if line.startswith("[") and line.endswith("]") and "wifi" in line.lower():
            if current.get("ssid"):
                profiles.append(current)
            current = {"ssid": "", "auth": "", "encryption": "", "password": ""}
        elif line.lower().startswith("ssid="):
            current["ssid"] = line.split("=", 1)[1].strip('"')
        elif line.lower().startswith("psk="):
            current["password"] = line.split("=", 1)[1].strip('"')
        elif line.lower().startswith("key-mgmt="):
            current["auth"] = line.split("=", 1)[1].strip()
    if current.get("ssid"):
        profiles.append(current)
    return profiles


def classify_auth(auth_str):
    a = auth_str.upper()
    if not a or a == "OPEN" or "OPEN" in a:
        return "Open"
    m = WPA_RE.search(auth_str)
    return m.group(0) if m else auth_str


def main():
    data = read_stdin()
    params = data.get("params", {})
    config = data.get("config", {})
    target_agent = params.get("agent_id", "")

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        agents = db.all_agents()
        if target_agent:
            agents = [a for a in agents if a["id"] == target_agent or a["id"][:8] == target_agent]

        agent_results = []
        total_profiles = 0
        total_with_pw = 0
        total_open = 0
        auth_counts = {}

        for agent in agents:
            tasks = db.tasks_for_agent(agent["id"])
            profiles = []
            for t in tasks:
                result_text = t.get("result", "") or ""
                cmd = (t.get("command", "") or "").lower()
                if "netsh wlan" in cmd or "wlan show profile" in cmd:
                    profiles.extend(parse_netsh_profiles(result_text))
                if "wpa_supplicant" in cmd or "networkmanager" in cmd or "nmcli" in cmd:
                    profiles.extend(parse_wpa_supplicant(result_text))
                    profiles.extend(parse_nm_connections(result_text))
                if "/etc/NetworkManager" in cmd:
                    profiles.extend(parse_nm_connections(result_text))
                if "key=clear" in cmd or "netsh wlan show profile" in cmd:
                    for m in KEY_RE.finditer(result_text):
                        for p in profiles:
                            if not p.get("password"):
                                p["password"] = m.group(1).strip()
                                break
            seen = set()
            deduped = []
            for p in profiles:
                key = p["ssid"]
                if key and key not in seen:
                    seen.add(key)
                    p["auth"] = classify_auth(p.get("auth", ""))
                    p["connected"] = bool(p.get("password"))
                    deduped.append(p)
            profiles = deduped

            agent_profiles = len(profiles)
            with_pw = sum(1 for p in profiles if p.get("password"))
            open_count = sum(1 for p in profiles if p["auth"] == "Open")

            total_profiles += agent_profiles
            total_with_pw += with_pw
            total_open += open_count
            for p in profiles:
                a_type = p["auth"]
                auth_counts[a_type] = auth_counts.get(a_type, 0) + 1

            agent_results.append({
                "id": agent["id"][:8],
                "hostname": agent.get("hostname", ""),
                "profiles": profiles,
            })

        output_lines = [
            f"Scanned {len(agents)} agents",
            f"{total_profiles} Wi-Fi profiles found",
            f"{total_with_pw} with passwords | {total_open} open networks",
        ]
        if auth_counts:
            top = sorted(auth_counts.items(), key=lambda x: -x[1])[:5]
            output_lines.append("Auth types: " + ", ".join(f"{k}={v}" for k, v in top))

        write_result(
            True,
            output="\n".join(output_lines),
            data={
                "total_agents": len(agents),
                "total_profiles": total_profiles,
                "with_password": total_with_pw,
                "open_networks": total_open,
                "by_auth": auth_counts,
                "agents": agent_results,
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
