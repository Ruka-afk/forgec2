#!/usr/bin/env python3
"""List backend routes missing from OpenAPI without external deps."""
import os
import re
import sys

route_re = re.compile(
    r'\.(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\(\s*["\x60]([^"\x60]+)["\x60]'
)
path_key_re = re.compile(r"^  (/[^:]+):\s*$")
method_re = re.compile(r"^    (get|post|put|delete|patch|head|options):\s*$", re.I)

backend = set()
for root, _dirs, files in os.walk("internal/server"):
    for f in files:
        if not f.endswith(".go") or f.endswith("_test.go"):
            continue
        text = open(os.path.join(root, f), encoding="utf-8", errors="ignore").read()
        for m in route_re.finditer(text):
            method = m.group(1).lower()
            p = m.group(2)
            np = re.sub(r":([A-Za-z_][A-Za-z0-9_]*)", r"{\1}", p)
            backend.add(f"{method} {np}")

specset = set()
cur_path = None
in_paths = False
for line in open("api/openapi.yaml", encoding="utf-8"):
    if line.startswith("paths:"):
        in_paths = True
        continue
    if in_paths and line.startswith("components:"):
        break
    if not in_paths:
        continue
    pm = path_key_re.match(line)
    if pm:
        cur_path = pm.group(1)
        continue
    mm = method_re.match(line)
    if mm and cur_path:
        specset.add(f"{mm.group(1).lower()} {cur_path}")

missing = sorted(backend - specset)
filter_keys = sys.argv[1:] if len(sys.argv) > 1 else []
shown = 0
for m in missing:
    if not filter_keys or any(k in m for k in filter_keys):
        print(m)
        shown += 1
print(
    f"--- shown {shown} total missing {len(missing)} documented {len(backend & specset)} backend {len(backend)}",
    file=sys.stderr,
)
