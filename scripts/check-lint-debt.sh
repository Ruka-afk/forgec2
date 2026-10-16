#!/usr/bin/env bash
# Lint debt gate, part of the `lint` CI job (see .github/workflows/ci.yml):
#   1. the mechanical linters (unconvert, ineffassign, govet, misspell) must
#      report zero findings on the whole tree, per analysis target;
#   2. the total backlog of a full uncapped run may only shrink, checked
#      against the ceiling in scripts/lint-backlog.txt.
#
# Inputs are golangci-lint JSON reports recorded with:
#   golangci-lint run ./... --max-issues-per-linter=0 --max-same-issues=0 \
#     --issues-exit-code=0 --output.json.path=<json> --output.text.path=<txt>
# once with GOOS=windows and once with GOOS=linux — the ceiling is keyed by
# target so the numbers do not depend on the host that ran the linter.
#
# Usage:
#   scripts/check-lint-debt.sh <windows.json> <linux.json>          # gate
#   scripts/check-lint-debt.sh --update <windows.json> <linux.json> # new ceiling
#
# --update rewrites the ceiling after debt is paid down. It refuses to raise a
# number: growing the backlog is never the answer.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

CAP_FILE=scripts/lint-backlog.txt

mode=check
if [ "${1:-}" = "--update" ]; then
    mode=update
    shift
fi

if [ "$#" -ne 2 ]; then
    echo "usage: $0 [--update] <windows.json> <linux.json>" >&2
    exit 2
fi

windows_json="$1"
linux_json="$2"

command -v jq >/dev/null 2>&1 || { echo "lint-debt: jq is required" >&2; exit 2; }

for json in "$windows_json" "$linux_json"; do
    if [ ! -f "$json" ]; then
        echo "lint-debt: report not found: $json" >&2
        exit 2
    fi
done

total_of() { jq '.Issues | length' "$1"; }

mechanical_of() {
    jq '[.Issues[]
         | select(.FromLinter == "unconvert" or .FromLinter == "ineffassign"
                  or .FromLinter == "govet" or .FromLinter == "misspell")] | length' "$1"
}

mechanical_list() {
    jq -r '.Issues[]
           | select(.FromLinter == "unconvert" or .FromLinter == "ineffassign"
                    or .FromLinter == "govet" or .FromLinter == "misspell")
           | "  \(.FromLinter) \(.Pos.Filename):\(.Pos.Line) \(.Text)"' "$1"
}

top_linters() {
    jq -r '[.Issues[].FromLinter] | group_by(.) | map({l: .[0], n: length})
           | sort_by(-.n) | .[] | "  \(.n)\t\(.l)"' "$1"
}

cap_of() {
    awk -F= -v key="$1" '$1 == key { gsub(/[[:space:]]/, "", $2); print $2 }' "$CAP_FILE"
}

check_target() {
    local target="$1" json="$2"
    local total mechanical cap
    total="$(total_of "$json")"
    mechanical="$(mechanical_of "$json")"
    cap="$(cap_of "$target")"

    if [ -z "$cap" ]; then
        echo "lint-debt: no '$target' ceiling in $CAP_FILE" >&2
        return 1
    fi
    if ! [[ "$total" =~ ^[0-9]+$ && "$mechanical" =~ ^[0-9]+$ && "$cap" =~ ^[0-9]+$ ]]; then
        echo "lint-debt($target): could not read counts (total='$total' mechanical='$mechanical' cap='$cap')" >&2
        return 1
    fi

    if [ "$mechanical" -ne 0 ]; then
        echo "lint-debt($target): mechanical linters must stay at zero, found $mechanical:" >&2
        mechanical_list "$json" >&2
        return 1
    fi

    echo "lint-debt($target): $total total, ceiling $cap"
    if [ "$total" -gt "$cap" ]; then
        echo "lint-debt($target): backlog grew by $((total - cap)) — pay it down or adjust the cleanup;" >&2
        echo "top linters:" >&2
        top_linters "$json" >&2
        return 1
    fi
    if [ "$total" -lt "$cap" ]; then
        echo "lint-debt($target): backlog shrank by $((cap - total)) — refresh $CAP_FILE" >&2
    fi
    return 0
}

if [ "$mode" = update ]; then
    new_windows="$(total_of "$windows_json" || true)"
    new_linux="$(total_of "$linux_json" || true)"
    if ! [[ "$new_windows" =~ ^[0-9]+$ && "$new_linux" =~ ^[0-9]+$ ]]; then
        echo "lint-debt: could not read counts (windows='$new_windows' linux='$new_linux')" >&2
        exit 1
    fi
    for pair in "windows:$new_windows" "linux:$new_linux"; do
        target="${pair%%:*}"
        value="${pair##*:}"
        cap="$(cap_of "$target")"
        if [ -z "$cap" ]; then
            echo "lint-debt: no '$target' ceiling in $CAP_FILE" >&2
            exit 1
        fi
        if [ "$value" -gt "$cap" ]; then
            echo "lint-debt: refusing to raise the $target ceiling ($cap -> $value)" >&2
            exit 1
        fi
    done
    for pair in "windows:$windows_json" "linux:$linux_json"; do
        target="${pair%%:*}"
        json="${pair#*:}"
        mechanical="$(mechanical_of "$json")"
        if [ "$mechanical" -ne 0 ]; then
            echo "lint-debt($target): $mechanical mechanical findings block an update" >&2
            mechanical_list "$json" >&2
            exit 1
        fi
    done
    awk -v w="$new_windows" -v l="$new_linux" '
        /^windows=/ { print "windows=" w; next }
        /^linux=/   { print "linux=" l;   next }
        { print }
    ' "$CAP_FILE" >"$CAP_FILE.tmp"
    mv "$CAP_FILE.tmp" "$CAP_FILE"
    echo "lint-debt: ceiling updated (windows=$new_windows, linux=$new_linux)"
    exit 0
fi

failed=0
check_target windows "$windows_json" || failed=1
check_target linux "$linux_json" || failed=1
if [ "$failed" -ne 0 ]; then
    echo "lint-debt: gate failed" >&2
    exit 1
fi
echo "lint-debt: OK (mechanical linters at zero, backlog within the ceiling)"
