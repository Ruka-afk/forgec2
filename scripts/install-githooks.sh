#!/usr/bin/env bash
# Installs the repository's git hooks (currently: gitleaks secret scan on
# commit). Idempotent; safe to re-run after updates.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HOOKS_DIR="$REPO_ROOT/.git/hooks"

mkdir -p "$HOOKS_DIR"

install_hook() {
  local name="$1"
  local src="$REPO_ROOT/scripts/$2"
  local dst="$HOOKS_DIR/$name"
  if [ -f "$dst" ] && [ ! -L "$dst" ]; then
    # Back up any existing hook (user may have configured their own).
    mv "$dst" "$dst.bak"
    echo "backed up existing hook to $dst.bak"
  fi
  cp "$src" "$dst"
  chmod +x "$dst"
  echo "installed $name hook -> $dst"
}

install_hook "pre-commit" "gitleaks-pre-commit.sh"
echo "hooks installed. To skip the scan for a single commit: git commit --no-verify"
