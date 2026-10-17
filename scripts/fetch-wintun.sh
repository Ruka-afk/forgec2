#!/usr/bin/env bash
# fetch-wintun.sh - stage the Wintun runtimes that Windows agents embed (CI/Linux).
# Mirror of scripts/fetch-wintun.ps1; keep the version + SHA256 in sync.
#
#   bash scripts/fetch-wintun.sh
#
# See fetch-wintun.ps1 for why the runtimes are fetched on demand instead of
# committed. Without staged runtimes the generator keeps the honest
# "no bundled Wintun" stub and no build breaks.
set -euo pipefail

WINTUN_VERSION="0.14.1"
WINTUN_ZIP_SHA256="07c256185d6ee3652e09fa55c0b673e2624b565e02c4b9091c79ca7d2f24ef51"
URL="https://www.wintun.net/builds/wintun-${WINTUN_VERSION}.zip"

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
assets_dir="$root/internal/payload/agent/assets/wintun"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Fetching wintun ${WINTUN_VERSION} ..."
curl -sSfL -o "$tmp/wintun.zip" "$URL"
echo "${WINTUN_ZIP_SHA256}  $tmp/wintun.zip" | sha256sum -c -

unzip -q "$tmp/wintun.zip" -d "$tmp"
for arch in amd64 arm64; do
  mkdir -p "$assets_dir/$arch"
  cp "$tmp/wintun/bin/$arch/wintun.dll" "$assets_dir/$arch/wintun.dll"
  echo "staged $assets_dir/$arch/wintun.dll"
done
cp "$tmp/wintun/LICENSE.txt" "$assets_dir/LICENSE.txt"
echo "Wintun ${WINTUN_VERSION} staged under internal/payload/agent/assets/wintun/"
