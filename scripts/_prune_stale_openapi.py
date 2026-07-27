#!/usr/bin/env python3
"""
Remove OpenAPI path keys whose methods are all absent from backend (-list-stale).

Usage:
  py -3 scripts/_prune_stale_openapi.py --dry-run
  py -3 scripts/_prune_stale_openapi.py
  py -3 scripts/_prune_stale_openapi.py --agentish-only   # legacy mode
"""
import re
import subprocess
import sys
from pathlib import Path

OPENAPI = Path("api/openapi.yaml")

AGENTISH = re.compile(
    r"^/(files/|keylogger/|clipboard/|prank/|screen/|rportfwd/|socks_relay/|"
    r"container_|mimikatz|creds|kerberoast|command$|beacon|browser_steal|"
    r"cookie_export|download|find$|drives$|net$|netstat|killproc|inject|"
    r"execute_assembly|powerpick|persistence|elevate|amsi_|etw_|av$|kill_av|"
    r"modules/deploy|import$|vpn_creds|wifi_creds|privesc_check|self_update|"
    r"spawn$|suspend$|resume$|reboot$|shutdown$|uninstall|set_sleep|run_evasion|"
    r"sandbox_detect|users$|services$|portscan$|bof$|lateral$|token/|"
    r"socks$|screenshot|ps$|reg/|coerce/)"
)


def stale_entries():
    proc = subprocess.run(
        ["go", "run", "./cmd/checkopenapi", "-list-stale"],
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
    )
    entries = []
    for line in (proc.stdout + "\n" + proc.stderr).splitlines():
        line = line.strip()
        m = re.match(r"^(get|post|put|delete|patch|head|options) (/.*)$", line)
        if m:
            entries.append((m.group(1), m.group(2)))
    return entries


def parse_path_blocks(text: str):
    lines = text.splitlines(keepends=True)
    starts = []
    in_paths = False
    components_at = len(lines)
    for i, line in enumerate(lines):
        if line.startswith("paths:"):
            in_paths = True
            continue
        if in_paths and line.startswith("components:"):
            components_at = i
            break
        if in_paths and re.match(r"^  /", line):
            starts.append(i)
    ends = starts[1:] + [components_at]
    blocks = []
    for s, e in zip(starts, ends):
        m = re.match(r"^  (/[^:]+):\s*$", lines[s])
        if m:
            blocks.append((s, e, m.group(1)))
    return blocks, lines


def main():
    dry = "--dry-run" in sys.argv
    agentish_only = "--agentish-only" in sys.argv
    stale = stale_entries()
    if not stale:
        print("no stale OpenAPI entries")
        return

    stale_by_path = {}
    for method, path in stale:
        stale_by_path.setdefault(path, set()).add(method)

    text = OPENAPI.read_text(encoding="utf-8")
    blocks, lines = parse_path_blocks(text)

    remove = []
    for start, end, path in blocks:
        methods_in_block = set()
        for j in range(start + 1, end):
            m = re.match(r"^    (get|post|put|delete|patch):\s*$", lines[j], re.I)
            if m:
                methods_in_block.add(m.group(1).lower())
        if not methods_in_block:
            continue
        stale_methods = stale_by_path.get(path, set())
        all_stale = methods_in_block <= stale_methods and len(stale_methods) > 0
        if not all_stale:
            continue
        if agentish_only and not AGENTISH.search(path):
            continue
        remove.append((start, end, path))

    if not remove:
        print(f"stale methods: {len(stale)}; no full path blocks fully orphaned")
        for method, path in stale[:40]:
            print(f"  {method} {path}")
        return

    print(f"removing {len(remove)} path blocks ({'dry-run' if dry else 'write'}):")
    for _, _, path in remove:
        print(f"  {path}")

    if dry:
        return

    remove.sort(reverse=True)
    for start, end, _ in remove:
        del lines[start:end]
    OPENAPI.write_text("".join(lines), encoding="utf-8", newline="\n")
    print(f"wrote {OPENAPI}")


if __name__ == "__main__":
    main()
