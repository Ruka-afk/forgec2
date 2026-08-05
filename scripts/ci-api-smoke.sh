#!/usr/bin/env bash
# Unauthenticated + optional default-admin authenticated API smoke (Linux CI).
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8000}"
COOKIE_JAR="$(mktemp)"
trap 'rm -f "$COOKIE_JAR"' EXIT

echo "=== ForgeC2 CI API smoke ($BASE_URL) ==="

# health
health="$(curl -sf "$BASE_URL/health")"
echo "$health" | grep -q '"status"[[:space:]]*:[[:space:]]*"ok"' || {
  echo "FAIL health"
  exit 1
}
echo "PASS health"

# modules unauthenticated → 401
code="$(curl -s -o /tmp/modules.body -w '%{http_code}' "$BASE_URL/api/modules")"
test "$code" = "401" || { echo "FAIL modules expected 401 got $code"; exit 1; }
grep -Eqi 'unauthorized|error|success' /tmp/modules.body || true
echo "PASS modules auth gate (401)"

# login SPA
code="$(curl -s -o /tmp/login.html -w '%{http_code}' "$BASE_URL/login")"
test "$code" = "200" || { echo "FAIL login SPA HTTP $code"; exit 1; }
test "$(wc -c < /tmp/login.html)" -gt 100 || { echo "FAIL login SPA empty"; exit 1; }
echo "PASS login SPA"

# Authenticated path with default admin (fresh seed)
curl -sf -c "$COOKIE_JAR" -b "$COOKIE_JAR" "$BASE_URL/login" -o /dev/null || true
CSRF="$(grep -E 'forgec2_csrf' "$COOKIE_JAR" | awk '{print $NF}' | tail -n1 || true)"

login_code="$(curl -s -o /tmp/login_post.body -w '%{http_code}' \
  -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
  -X POST "$BASE_URL/login" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -H "Accept: application/json" \
  ${CSRF:+-H "X-CSRF-Token: $CSRF"} \
  --data "username=admin&password=Admin123!" \
  --max-redirs 0 || true)"

# 302 / 200 / 0-style redirect all OK if session cookie present
has_session=0
if grep -q 'forgec2_session' "$COOKIE_JAR" 2>/dev/null; then
  has_session=1
fi

if [[ "$login_code" =~ ^(200|301|302|303|307)$ ]] || [[ "$has_session" -eq 1 ]]; then
  echo "PASS login (HTTP $login_code session=$has_session)"
else
  echo "FAIL login HTTP $login_code session=$has_session (expect admin/Admin123! from auth.default_password on fresh DB)"
  cat /tmp/login_post.body 2>/dev/null || true
  tail -n 30 /tmp/forgec2-smoke.log 2>/dev/null || true
  exit 1
fi

CSRF="$(grep -E 'forgec2_csrf' "$COOKIE_JAR" | awk '{print $NF}' | tail -n1 || true)"
AUTH_H=(-b "$COOKIE_JAR" -H "Accept: application/json")
if [[ -n "${CSRF:-}" ]]; then
  AUTH_H+=(-H "X-CSRF-Token: $CSRF")
fi

for path in /agents /api/modules /api/listeners /api/v1/dashboard /api/v1/task-types /api/generate/profiles; do
  code="$(curl -s -o /tmp/auth_body -w '%{http_code}' "${AUTH_H[@]}" "$BASE_URL$path")"
  if [[ "$code" == "200" ]]; then
    echo "PASS GET $path"
  else
    echo "FAIL GET $path HTTP $code"
    head -c 200 /tmp/auth_body || true
    exit 1
  fi
done

echo "=== All smoke checks passed ==="
