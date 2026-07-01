#!/usr/bin/env python3
"""ForgeC2 portscan plugin.

Reads a JSON object from stdin with the following fields:
    agent_id (str): ID of the agent the command is running for.
    params (dict): Contains target, ports and timeout.
    config (dict): Plugin configuration.

Writes a JSON Result object to stdout.
"""

import json
import socket
import sys
from typing import Any


def scan_port(target: str, port: int, timeout: float) -> bool:
    try:
        with socket.create_connection((target, port), timeout=timeout):
            return True
    except OSError:
        return False


def main() -> None:
    raw = sys.stdin.read()
    if not raw:
        result = {"success": False, "error": "empty input"}
        print(json.dumps(result))
        return

    try:
        data = json.loads(raw)
    except json.JSONDecodeError as exc:
        result = {"success": False, "error": f"invalid json: {exc}"}
        print(json.dumps(result))
        return

    params = data.get("params", {})
    target = params.get("target", "")
    ports_raw = params.get("ports", "80,443,445,3389")
    timeout = float(params.get("timeout", 1))
    config = data.get("config", {})

    if not target:
        print(json.dumps({"success": False, "error": "target is required"}))
        return

    max_ports = int(config.get("max_ports", 100))

    open_ports = []
    try:
        ports = []
        for part in str(ports_raw).split(","):
            part = part.strip()
            if not part:
                continue
            if "-" in part:
                start, end = part.split("-", 1)
                ports.extend(range(int(start), int(end) + 1))
            else:
                ports.append(int(part))
    except ValueError as exc:
        print(json.dumps({"success": False, "error": f"invalid ports: {exc}"}))
        return

    if len(ports) > max_ports:
        ports = ports[:max_ports]

    for port in ports:
        if scan_port(target, port, timeout):
            open_ports.append(port)

    result = {
        "success": True,
        "data": {
            "target": target,
            "scanned": len(ports),
            "open_ports": open_ports,
        },
        "output": f"Scanned {len(ports)} port(s) on {target}, {len(open_ports)} open.",
    }
    print(json.dumps(result))


if __name__ == "__main__":
    main()
