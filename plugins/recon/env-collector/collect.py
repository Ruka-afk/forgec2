#!/usr/bin/env python3
"""Env Collector plugin — parses task results to extract environment variables, system configuration, and dev tools."""

import json
import os
import re
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

CREDENTIAL_KEYS = {
    "AWS_SECRET_ACCESS_KEY", "AWS_ACCESS_KEY_ID", "DATABASE_URL",
    "MYSQL_PWD", "POSTGRES_PASSWORD", "REDIS_PASSWORD",
    "API_KEY", "SECRET_KEY", "PRIVATE_KEY", "TOKEN",
    "GITHUB_TOKEN", "NPM_TOKEN",
}

PATH_KEYS = {
    "PATH", "GOPATH", "JAVA_HOME", "PYTHONPATH", "NODE_PATH",
}

PROXY_KEYS = {"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY"}

NETWORK_KEYS = {"HOSTNAME", "DOMAIN", "COMPUTERNAME"}

DEV_TOOL_PATTERNS = {
    "Git": re.compile(r"git\s+version\s+git-(\S+)", re.IGNORECASE),
    "Python": re.compile(r"Python\s+(\S+)"),
    "Node": re.compile(r"v(\d+\.\d+\.\d+)"),
    "Go": re.compile(r"go version\s+\S+\s+(\S+)"),
    "Java": re.compile(r"openjdk\s+version\s+\"(\S+)\"", re.IGNORECASE),
    "Ruby": re.compile(r"ruby\s+(\S+)"),
    "Docker": re.compile(r"Docker\s+version\s+(\S+)", re.IGNORECASE),
    "NPM": re.compile(r"npm\s+version\s+(\S+)"),
}

ENV_TASK_CMDS = ("set", "env", "Get-ChildItem Env:", "printenv")
SHELL_CONFIG_CMDS = ("/etc/environment", ".bashrc", ".profile", ".bash_profile")
DOCKER_CMD = "docker inspect"


def parse_env_lines(text: str) -> dict:
    """Parse KEY=VALUE or NAME    Value lines into a dict."""
    env = {}
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        # Windows-style: NAME=VALUE (from `set`)
        if "=" in line and not line.startswith(" "):
            parts = line.split("=", 1)
            if len(parts) == 2:
                env[parts[0].strip()] = parts[1].strip()
        # PowerShell-style: Name    Value
        m = re.match(r"^(\S+)\s{2,}(.+)$", line)
        if m:
            env[m.group(1)] = m.group(2).strip()
    return env


def parse_docker_inspect(text: str) -> list:
    """Extract Env arrays from docker inspect JSON output."""
    container_envs = []
    try:
        data = json.loads(text) if text.strip().startswith("[") else [json.loads(text)]
    except (json.JSONDecodeError, ValueError):
        return container_envs

    for container in data:
        if not isinstance(container, dict):
            continue
        name = container.get("Name", "").lstrip("/")
        config = container.get("Config") or {}
        env_list = config.get("Env") or []
        env_dict = {}
        for entry in env_list:
            if "=" in entry:
                k, v = entry.split("=", 1)
                env_dict[k] = v
        if env_dict:
            container_envs.append({"name": name, "env": env_dict})
    return container_envs


def detect_dev_tools(text: str) -> list:
    """Detect dev tool versions from command output."""
    tools = []
    for name, pattern in DEV_TOOL_PATTERNS.items():
        m = pattern.search(text)
        if m:
            tools.append({"name": name, "version": m.group(1)})
    return tools


def classify_env(env: dict) -> dict:
    """Classify env vars into categories."""
    credentials = []
    paths = {}
    proxy = {}
    network = {}

    for key, value in env.items():
        upper = key.upper()
        if upper in CREDENTIAL_KEYS:
            preview = value[:4] + "****" if len(value) > 4 else "****"
            credentials.append({"name": key, "value_preview": preview})
        elif upper in PATH_KEYS:
            if upper == "PATH":
                paths["PATH"] = [p.strip() for p in value.split(";") if p.strip()] if ";" in value else [p.strip() for p in value.split(":") if p.strip()]
            else:
                paths[key] = value
        elif upper in PROXY_KEYS:
            proxy[key] = value
        elif upper in NETWORK_KEYS:
            network[key] = value

    return {"credentials": credentials, "paths": paths, "proxy": proxy, "network": network}


def main():
    data = read_stdin()
    params = data.get("params", {})
    agent_id = params.get("agent_id", "")

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        tasks = db.tasks_for_agent(agent_id) if agent_id else db.all_tasks()
        env_tasks = [t for t in tasks if t.get("status") == "completed"]

        agent_results = []
        total_creds = 0
        all_dev_tools = set()
        all_paths = {}

        # Group tasks by agent
        agent_tasks = {}
        for task in env_tasks:
            aid = task.get("agent_id", "")
            if aid not in agent_tasks:
                agent_tasks[aid] = []
            agent_tasks[aid].append(task)

        target_agents = [agent_id] if agent_id else list(agent_tasks.keys())

        for aid in target_agents:
            if aid not in agent_tasks:
                continue

            agent = db.agent_by_id(aid)
            hostname = agent.get("hostname", "unknown") if agent else "unknown"

            combined_env = {}
            dev_tools = []
            container_envs = []

            for task in agent_tasks[aid][:15]:
                result_text = task.get("result", "")
                command = (task.get("command", "") or "").lower()

                if not result_text:
                    continue

                # Detect env var output
                if any(cmd in command for cmd in ENV_TASK_CMDS):
                    combined_env.update(parse_env_lines(result_text))
                    dev_tools.extend(detect_dev_tools(result_text))

                # Shell config files
                elif any(cfg in command for cfg in SHELL_CONFIG_CMDS):
                    combined_env.update(parse_env_lines(result_text))

                # Docker inspect
                elif DOCKER_CMD in command:
                    container_envs.extend(parse_docker_inspect(result_text))
                    dev_tools.extend(detect_dev_tools(result_text))

                # Generic shell task — try to detect dev tools from any output
                else:
                    dev_tools.extend(detect_dev_tools(result_text))

            classified = classify_env(combined_env)

            # Add container envs as separate credential entries
            for ce in container_envs:
                for k, v in ce["env"].items():
                    upper = k.upper()
                    if upper in CREDENTIAL_KEYS:
                        preview = v[:4] + "****" if len(v) > 4 else "****"
                        classified["credentials"].append({
                            "name": f"{ce['name']}:{k}",
                            "value_preview": preview,
                        })

            # Deduplicate dev tools
            seen_tools = set()
            unique_tools = []
            for t in dev_tools:
                key = (t["name"], t["version"])
                if key not in seen_tools:
                    seen_tools.add(key)
                    unique_tools.append(t)
                    all_dev_tools.add(t["name"])

            total_creds += len(classified["credentials"])

            # Track paths
            if classified["paths"]:
                for k, v in classified["paths"].items():
                    if k not in all_paths:
                        all_paths[k] = set()
                    if isinstance(v, list):
                        all_paths[k].update(v)
                    else:
                        all_paths[k].add(str(v))

            agent_results.append({
                "id": aid,
                "hostname": hostname,
                "credentials": classified["credentials"],
                "paths": {k: v if isinstance(v, list) else [v] for k, v in classified["paths"].items()},
                "dev_tools": unique_tools,
                "proxy": classified["proxy"],
                "network": classified["network"],
            })

        # Build common paths (paths found in >1 agent)
        common_paths = []
        for key, values in all_paths.items():
            if len(agent_results) > 1 and len(values) > 1:
                common_paths.append(key)
            elif len(agent_results) == 1:
                common_paths.append(key)

        summary = {
            "total_agents": len(agent_results),
            "credentials_found": total_creds,
            "dev_tools_unique": sorted(all_dev_tools),
            "common_paths": common_paths,
        }

        output = (
            f"Scanned {len(agent_results)} agents | "
            f"{total_creds} credentials in env | "
            f"{len(all_dev_tools)} dev tools"
        )

        write_result(
            True,
            output=output,
            data={"agents": agent_results, "summary": summary},
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
