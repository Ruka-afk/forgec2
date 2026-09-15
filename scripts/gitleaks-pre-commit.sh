#!/usr/bin/env bash
# Pre-commit hook: run gitleaks over staged files to keep secrets out of the
# repository. Installed by scripts/install-githooks.sh.
set -euo pipefail

if ! command -v gitleaks >/dev/null 2>&1; then
  echo "gitleaks: not found — skipping secret scan (install via 'go install github.com/gitleaks/gitleaks/v8@latest')." >&2
  exit 0
fi

cd "$(git rev-parse --show-toplevel)"

# Scan the whole working tree (cheap for this repo) so moves/renames of files
# that carried secrets are caught too.
# Prefer the repo .gitleaks.toml (project-specific rules) when present.
GITLEAKS_ARGS="--redact --no-banner --source ."
if [ -f ".gitleaks.toml" ]; then
  GITLEAKS_ARGS="--config .gitleaks.toml $GITLEAKS_ARGS"
fi
if ! gitleaks detect $GITLEAKS_ARGS 2>/dev/null; then
  echo "gitleaks: secret material detected — refusing to commit." >&2
  echo "Remove the secret, then 'git add' the fixed file and retry." >&2
  exit 1
fi

exit 0
