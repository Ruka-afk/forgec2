#!/usr/bin/env python3
"""Credential Analyzer plugin — analyzes collected credentials for patterns and high-value accounts."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

HIGH_VALUE_DEFAULT = "Administrator,admin,root,sa,krbtgt"


def main():
    data = read_stdin()
    config = data.get("config", {})
    high_value_str = config.get("high_value_users", HIGH_VALUE_DEFAULT)
    high_value_users = {u.strip().lower() for u in high_value_str.split(",") if u.strip()}

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        creds = db.all_credentials()

        # Type breakdown
        type_counts = {}
        for c in creds:
            t = c.get("type") or "unknown"
            type_counts[t] = type_counts.get(t, 0) + 1

        # Source breakdown
        source_counts = {}
        for c in creds:
            s = c.get("source") or "unknown"
            source_counts[s] = source_counts.get(s, 0) + 1

        # Domain breakdown
        domain_counts = {}
        for c in creds:
            d = c.get("domain") or "N/A"
            domain_counts[d] = domain_counts.get(d, 0) + 1

        # High-value accounts
        high_value = []
        for c in creds:
            username = (c.get("username") or "").lower()
            for hv in high_value_users:
                if hv in username:
                    high_value.append({
                        "id": c.get("id"),
                        "username": c.get("username"),
                        "domain": c.get("domain"),
                        "type": c.get("type"),
                        "source": c.get("source"),
                        "confirmed": c.get("confirmed"),
                        "created_at": str(c.get("created_at")),
                    })
                    break

        # Duplicate detection (same username+domain, different sources)
        seen_pairs = {}
        duplicates = []
        for c in creds:
            key = ((c.get("username") or "").lower(), (c.get("domain") or "").lower())
            if key not in seen_pairs:
                seen_pairs[key] = []
            seen_pairs[key].append(c)
        for pair, entries in seen_pairs.items():
            if len(entries) > 1:
                sources = list({e.get("source") or "unknown" for e in entries})
                if len(sources) > 1:
                    duplicates.append({
                        "username": pair[0],
                        "domain": pair[1],
                        "count": len(entries),
                        "sources": sources,
                    })

        # Confirmed vs unconfirmed
        confirmed = sum(1 for c in creds if c.get("confirmed"))
        unconfirmed = len(creds) - confirmed

        # Tags summary
        tag_counts = {}
        for c in creds:
            tags = (c.get("tags") or "").split(",")
            for tag in tags:
                tag = tag.strip()
                if tag:
                    tag_counts[tag] = tag_counts.get(tag, 0) + 1

        output_lines = [
            f"Total credentials: {len(creds)}",
            f"Confirmed: {confirmed} | Unconfirmed: {unconfirmed}",
            f"Types: {', '.join(f'{k}={v}' for k, v in sorted(type_counts.items(), key=lambda x: -x[1]))}",
            f"Sources: {', '.join(f'{k}={v}' for k, v in sorted(source_counts.items(), key=lambda x: -x[1]))}",
            f"High-value accounts: {len(high_value)}",
            f"Duplicates (different sources): {len(duplicates)}",
        ]

        write_result(
            True,
            output="\n".join(output_lines),
            data={
                "total": len(creds),
                "confirmed": confirmed,
                "unconfirmed": unconfirmed,
                "type_counts": type_counts,
                "source_counts": source_counts,
                "domain_counts": domain_counts,
                "high_value": high_value,
                "duplicates": duplicates,
                "tag_counts": tag_counts,
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
