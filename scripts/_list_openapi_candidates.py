#!/usr/bin/env python3
import os
import re

route_re = re.compile(
    r'\.(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\(\s*["\x60]([^"\x60]+)["\x60]'
)
backend = set()
for root, _, files in os.walk("internal/server"):
    for f in files:
        if not f.endswith(".go") or f.endswith("_test.go"):
            continue
        text = open(os.path.join(root, f), encoding="utf-8", errors="ignore").read()
        for m in route_re.finditer(text):
            p = re.sub(r":([A-Za-z_][A-Za-z0-9_]*)", r"{\1}", m.group(2))
            backend.add(f"{m.group(1).lower()} {p}")

spec = set()
cur = None
for line in open("api/openapi.yaml", encoding="utf-8"):
    pm = re.match(r"^  (/[^:]+):\s*$", line)
    if pm:
        cur = pm.group(1)
        continue
    mm = re.match(r"^    (get|post|put|delete|patch):\s*$", line, re.I)
    if mm and cur:
        spec.add(f"{mm.group(1).lower()} {cur}")

miss = sorted(backend - spec)
skip_sub = (
    "pprof",
    "goroutine",
    "heap",
    "mutex",
    "threadcreate",
    "trace",
    "symbol",
    "block",
    "cmdline",
)
good = []
for m in miss:
    method, path = m.split(" ", 1)
    segs = [s for s in path.split("/") if s]
    if any(x in path for x in skip_sub):
        continue
    if method == "get" and len(segs) <= 1 and segs and "{" not in path:
        continue
    good.append(m)

print(
    f"missing={len(miss)} candidates={len(good)} documented={len(backend & spec)} backend={len(backend)}"
)
for g in good:
    print(g)
