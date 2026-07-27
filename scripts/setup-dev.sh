#!/usr/bin/env bash
set -euo pipefail

# ForgeC2 — Development Environment Setup
# Usage: bash scripts/setup-dev.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$ROOT_DIR"

echo "=== ForgeC2 Development Setup ==="
echo ""

# Check prerequisites
check_cmd() {
  if ! command -v "$1" &>/dev/null; then
    echo "ERROR: $1 is required but not installed."
    echo "  $2"
    return 1
  fi
}

check_cmd go "https://go.dev/dl/"
check_cmd node "https://nodejs.org/"
check_cmd npm "https://nodejs.org/"

echo "Go:   $(go version)"
echo "Node: $(node --version)"
echo "npm:  $(npm --version)"
echo ""

# Create config.yaml if not present
if [ ! -f config.yaml ]; then
  echo "Creating default config.yaml..."
  JWT_SECRET=$(head -c 32 /dev/urandom | base64 | tr -d '/+=' | head -c 32)
  cat > config.yaml <<EOF
server:
  port: 8000
  host: "127.0.0.1"
  tls_enabled: false
  jwt_secret: "$JWT_SECRET"
  data_dir: "data"

database:
  path: "data/db/forgec2.db"
  driver: "sqlite"

implant:
  default_interval: 10
  default_jitter: 25

crypto:
  key: "xorc2key123456789012345678901234"

log:
  level: "info"
  file: ""
EOF
  echo "  config.yaml created (edit to taste)"
else
  echo "config.yaml already exists, skipping."
fi
echo ""

# Create data directories
echo "Creating data directories..."
mkdir -p data/db
mkdir -p data/loot
mkdir -p data/screenshots
mkdir -p data/payloads
mkdir -p data/logs
echo "  data/ directories ready"
echo ""

# Install Go dependencies
echo "Installing Go dependencies..."
go mod download 2>/dev/null || go mod tidy
echo "  Go dependencies ready"
echo ""

# Install frontend dependencies and build
echo "Installing frontend dependencies..."
cd frontend
npm ci 2>/dev/null || npm install
echo ""
echo "Building frontend..."
npm run build
cd "$ROOT_DIR"
echo "  Frontend built"
echo ""

# Copy frontend to embedded dist
echo "Copying frontend to embedded dist..."
rm -rf internal/webdist/dist
cp -r frontend/out internal/webdist/dist
echo "  Embedded dist ready"
echo ""

# Build the server binary
echo "Building forgec2-server..."
go build -ldflags="-s -w" -o forgec2-server.exe ./cmd/server
echo "  Binary ready: forgec2-server.exe"
echo ""

# Run tests
echo "Running tests..."
go test ./... -count=1 -timeout 120s 2>&1 || true
echo ""

echo "=== Setup Complete ==="
echo ""
echo "Start the server:"
echo "  ./forgec2-server.exe -config config.yaml"
echo ""
echo "Or use the dev script:"
echo "  powershell -File scripts/dev-backend.ps1"
