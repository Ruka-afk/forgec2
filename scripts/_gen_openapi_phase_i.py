#!/usr/bin/env python3
"""Phase I: document full-prefixed routes now extracted by checkopenapi."""
import re
import subprocess
from collections import OrderedDict

OPENAPI = "api/openapi.yaml"


def missing_routes():
    out = subprocess.check_output(
        ["go", "run", "./cmd/checkopenapi", "-list-missing"],
        stderr=subprocess.STDOUT,
        text=True,
        encoding="utf-8",
        errors="replace",
    )
    routes = []
    for line in out.splitlines():
        line = line.strip()
        if re.match(r"^(get|post|put|delete|patch) /", line):
            routes.append(line)
    return routes


def load_existing(text: str):
    existing = set()
    path_keys = set()
    cur = None
    for line in text.splitlines():
        m = re.match(r"^  (/[^:]+):\s*$", line)
        if m:
            cur = m.group(1)
            path_keys.add(cur)
            continue
        mm = re.match(r"^    (get|post|put|delete|patch):\s*$", line, re.I)
        if mm and cur:
            existing.add(f"{mm.group(1).lower()} {cur}")
    return existing, path_keys


def to_openapi_path(path: str) -> str:
    segs = path.split("/")
    out = []
    for s in segs:
        if s.startswith(":"):
            out.append("{" + s[1:] + "}")
        else:
            out.append(s)
    return "/".join(out)


def tag_for(path: str) -> str:
    if path.startswith("/agents"):
        return "Agents"
    if path.startswith("/api/plugins"):
        return "Plugins"
    if path.startswith("/api/v1"):
        return "API"
    if path.startswith("/debug"):
        return "Monitor"
    if path.startswith("/toolkit"):
        return "Agents"
    return "Misc"


def method_block(method, tag, summary, opid, path):
    pnames = re.findall(r"\{([^}]+)\}", path)
    lines = [
        f"    {method}:",
        f"      tags: [{tag}]",
        f"      summary: {summary}",
        f"      operationId: {opid}",
        "      security:",
        "        - SessionCookie: []",
    ]
    if pnames:
        lines.append("      parameters:")
        for pname in pnames:
            lines.append(f"        - name: {pname}")
            lines.append("          in: path")
            lines.append("          required: true")
            lines.append("          schema: { type: string }")
    lines.append("      responses:")
    lines.append("        '200':")
    lines.append("          description: OK")
    return "\n".join(lines) + "\n"


def opid(method, path: str) -> str:
    # stable-ish id
    p = path.strip("/").replace("/", "_").replace("{", "").replace("}", "").replace("-", "_")
    return f"{method}_{p}"[:80]


def main():
    miss = missing_routes()
    # Skip pure SPA pages and noisy debug
    skip_exact = {
        "get /",
        "get /dashboard",
        "get /login",
        "get /generate",
        "get /lateral",
        "get /privesc",
        "get /scanner",
        "get /report",
        "get /plugins",
        "get /pivoting",
        "get /scripting",
        "get /toolkit",
        "get /topology",
        "get /traffic",
        "get /dns",
        "get /docs",
        "get /infrastructure",
        "get /automation",
        "get /search",
        "get /ai",
        "get /bof_repo",
        "get /templates",
        "get /translations",
        "get /metrics",
        "get /th",
        "get /generate_204",
    }
    candidates = []
    for m in miss:
        if m in skip_exact:
            continue
        method, path = m.split(" ", 1)
        if path.startswith("/debug/"):
            continue  # optional; omit pprof from public OpenAPI
        candidates.append((method, path))

    text = open(OPENAPI, encoding="utf-8").read()
    existing, path_keys = load_existing(text)

    by_path = OrderedDict()
    added = 0
    for method, path in candidates:
        opath = to_openapi_path(path)
        key = f"{method} {opath}"
        # also skip if backend-style already documented via conversion
        if key in existing:
            continue
        if opath in path_keys:
            # path exists - only add if method missing: skip for simplicity (avoid merge complexity)
            # Check method on existing path key
            if f"{method} {opath}" in existing:
                continue
            # If path key exists with other methods, skip to avoid YAML key collision
            continue
        by_path.setdefault(opath, []).append(method)
        added += 1  # provisional

    # rebuild with unique methods
    chunks = []
    real_added = 0
    for opath, methods in by_path.items():
        chunk = f"  {opath}:\n"
        seen_m = set()
        for method in methods:
            if method in seen_m:
                continue
            seen_m.add(method)
            tag = tag_for(opath)
            summary = f"{method.upper()} {opath}"
            chunk += method_block(method, tag, summary, opid(method, opath), opath)
            real_added += 1
        chunks.append(chunk)

    if not chunks:
        print("nothing to add")
        return

    insert = "\n" + "\n".join(chunks) + "\n"
    marker = "  /api/v1/dashboard:"
    if marker not in text:
        raise SystemExit("marker not found")
    text = text.replace(marker, insert + marker, 1)
    open(OPENAPI, "w", encoding="utf-8", newline="\n").write(text)
    print(f"added {real_added} methods across {len(chunks)} paths from {len(candidates)} candidates")


if __name__ == "__main__":
    main()
