#!/usr/bin/env bash
# Full-stack live-smoke harness: boots a throwaway Postgres, migrates+seeds,
# starts the API and the web dev server, runs Playwright, and tears everything
# down on every exit path. Run from anywhere: `bash apps/web/e2e/run-stack.sh`.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
PG_CONTAINER="mindimprint-e2e-pg"
PG_PORT="${E2E_PG_PORT:-55432}"
DB_URL="postgres://postgres:postgres@localhost:${PG_PORT}/mindimprint?sslmode=disable"
API_PID=""
WEB_PID=""

# Kill any process currently listening on a TCP port (macOS-compatible via lsof).
# Used both in cleanup (belt-and-suspenders) and as the fallback port-occupancy probe.
kill_port_listener() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1; then
    local pids
    pids=$(lsof -nP -iTCP:"${port}" -sTCP:LISTEN -t 2>/dev/null) || true
    if [ -n "$pids" ]; then
      # shellcheck disable=SC2086
      kill $pids 2>/dev/null || true
    fi
  fi
}

cleanup() {
  set +e
  # Kill subshell + its direct children (go run / pnpm).
  # On macOS without setsid, pkill -P kills direct children (the go run / pnpm
  # process), then we kill the subshell itself.  The compiled API binary or
  # vite/node process may survive reparented to launchd, so we follow up with
  # kill_port_listener which uses lsof to find and kill any remaining listener.
  if [ -n "$WEB_PID" ]; then
    pkill -P "$WEB_PID" 2>/dev/null || true
    kill "$WEB_PID" 2>/dev/null || true
  fi
  if [ -n "$API_PID" ]; then
    pkill -P "$API_PID" 2>/dev/null || true
    kill "$API_PID" 2>/dev/null || true
  fi
  # Belt-and-suspenders: kill any listener that survived the above.
  kill_port_listener 5173
  kill_port_listener 8080
  docker rm -f "$PG_CONTAINER" >/dev/null 2>&1
}
trap cleanup EXIT INT TERM

# Abort if a port is already occupied so we never silently run against a stale stack.
check_port_free() {
  local port="$1"
  local occupied=false
  if command -v lsof >/dev/null 2>&1; then
    if lsof -nP -iTCP:"${port}" -sTCP:LISTEN -t >/dev/null 2>&1; then
      occupied=true
    fi
  else
    # lsof unavailable: treat an HTTP 200 as "occupied".
    if curl -sf --connect-timeout 1 "http://localhost:${port}" >/dev/null 2>&1; then
      occupied=true
    fi
  fi
  if $occupied; then
    echo "FATAL: port ${port} already in use (stale stack? kill it and retry)"
    exit 1
  fi
}

command -v docker >/dev/null 2>&1 || { echo "FATAL: Docker is required for the throwaway Postgres."; exit 1; }
[ -f "$REPO/apps/api/.env.local" ] || { echo "FATAL: create apps/api/.env.local with DEEPSEEK_API_KEY (see RUNBOOK.md)."; exit 1; }
grep -qE '^DEEPSEEK_API_KEY=.+' "$REPO/apps/api/.env.local" || echo "WARN: DEEPSEEK_API_KEY looks empty — live-model steps will fail."

echo "==> Booting throwaway Postgres on :${PG_PORT}"
docker rm -f "$PG_CONTAINER" >/dev/null 2>&1 || true
docker run -d --name "$PG_CONTAINER" -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=mindimprint -p "${PG_PORT}:5432" postgres:16 >/dev/null
until docker exec "$PG_CONTAINER" pg_isready -U postgres >/dev/null 2>&1; do sleep 0.5; done

echo "==> Applying migrations + seed"
( cd "$REPO/apps/api" && DATABASE_URL="$DB_URL" go run ./cmd/api -migrate-up )

echo "==> Checking port 8080 is free"
check_port_free 8080

echo "==> Starting API on :8080"
# DATABASE_URL/COOKIE_SECURE/CORS_ORIGINS are exported here; godotenv does NOT
# override already-set env vars, so .env.local still supplies DEEPSEEK_API_KEY.
( cd "$REPO/apps/api" && DATABASE_URL="$DB_URL" COOKIE_SECURE=false CORS_ORIGINS=http://localhost:5173 go run ./cmd/api ) &
API_PID=$!
_wait_iters=0
until curl -sf http://localhost:8080/healthz >/dev/null 2>&1; do
  sleep 0.5
  _wait_iters=$((_wait_iters + 1))
  if [ "$_wait_iters" -ge 120 ]; then
    echo "FATAL: API never became ready on :8080 after 60s"
    exit 1
  fi
done

echo "==> Checking port 5173 is free"
check_port_free 5173

echo "==> Starting web dev server on :5173"
( cd "$REPO" && pnpm --filter web dev --port 5173 --strictPort ) &
WEB_PID=$!
_wait_iters=0
until curl -sf http://localhost:5173 >/dev/null 2>&1; do
  sleep 0.5
  _wait_iters=$((_wait_iters + 1))
  if [ "$_wait_iters" -ge 120 ]; then
    echo "FATAL: web dev server never became ready on :5173 after 60s"
    exit 1
  fi
done

echo "==> Running Playwright"
( cd "$REPO/apps/web" && pnpm e2e "$@" )
