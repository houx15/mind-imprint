#!/usr/bin/env bash
# Lite-edition live-walk harness: boots a throwaway Postgres, migrates, starts
# the API and the LITE dev server, runs Playwright, and tears everything down
# on every exit path. Run from anywhere: `bash apps/lite-web/e2e/run-stack.sh`.
#
# Same shape as apps/web/e2e/run-stack.sh, with three deliberate differences:
#   - its own Postgres container + port, so it never collides with the pro
#     harness or with a dev database;
#   - the web dev server is the LITE one, on :5174 (pro owns :5173);
#   - the school is flipped to `edition='lite'` in globalSetup, not here —
#     every /api/v1/readings route is gated on it (requireEdition), so the
#     seed belongs with the sign-in, where a failure is loud.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
PG_CONTAINER="${E2E_PG_CONTAINER:-mindimprint-lite-e2e-pg}"
PG_PORT="${E2E_PG_PORT:-55433}"
# 可覆盖，默认不变。用得上它的场合：机器上已经有别的 postgres 标签（比如
# postgres:16-alpine），而 registry 拉不动——Docker Desktop 的钥匙串凭据助手坏掉
# 时，任何一次 pull 都会以 `error getting credentials (-50)` 失败，本地已有的镜像
# 却照常能跑。
PG_IMAGE="${E2E_PG_IMAGE:-postgres:16}"
WEB_PORT="${E2E_WEB_PORT:-5174}"
# 8080 常被别的项目占着（本机上就有）。杀掉别人的服务不是我们该做的事，
# 换一个端口就行。
API_PORT="${E2E_API_PORT:-8080}"
DB_URL="postgres://postgres:postgres@localhost:${PG_PORT}/mindimprint?sslmode=disable"
API_PID=""
WEB_PID=""

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
  if [ -n "$WEB_PID" ]; then
    pkill -P "$WEB_PID" 2>/dev/null || true
    kill "$WEB_PID" 2>/dev/null || true
  fi
  if [ -n "$API_PID" ]; then
    pkill -P "$API_PID" 2>/dev/null || true
    kill "$API_PID" 2>/dev/null || true
  fi
  kill_port_listener "$WEB_PORT"
  kill_port_listener "$API_PORT"
  docker rm -f "$PG_CONTAINER" >/dev/null 2>&1
}
trap cleanup EXIT INT TERM

check_port_free() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1; then
    if lsof -nP -iTCP:"${port}" -sTCP:LISTEN -t >/dev/null 2>&1; then
      echo "FATAL: port ${port} already in use (stale stack? kill it and retry)"
      exit 1
    fi
  fi
}

command -v docker >/dev/null 2>&1 || { echo "FATAL: Docker is required for the throwaway Postgres."; exit 1; }
[ -f "$REPO/apps/api/.env.local" ] || { echo "FATAL: create apps/api/.env.local (see apps/web/e2e/RUNBOOK.md)."; exit 1; }
grep -qE '^DEEPSEEK_API_KEY=.+' "$REPO/apps/api/.env.local" || echo "WARN: DEEPSEEK_API_KEY looks empty — the live-lens walk will fail."

echo "==> Booting throwaway Postgres on :${PG_PORT} (${PG_IMAGE})"
docker rm -f "$PG_CONTAINER" >/dev/null 2>&1 || true
docker run -d --name "$PG_CONTAINER" -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=mindimprint -p "${PG_PORT}:5432" "$PG_IMAGE" >/dev/null
until docker exec "$PG_CONTAINER" pg_isready -U postgres >/dev/null 2>&1; do sleep 0.5; done

echo "==> Applying migrations + seed"
( cd "$REPO/apps/api" && DATABASE_URL="$DB_URL" go run ./cmd/api -migrate-up )

echo "==> Checking port ${API_PORT} is free"
check_port_free "$API_PORT"

echo "==> Starting API on :${API_PORT}"
( cd "$REPO/apps/api" && PORT="$API_PORT" DATABASE_URL="$DB_URL" COOKIE_SECURE=false CORS_ORIGINS="http://localhost:${WEB_PORT}" go run ./cmd/api ) &
API_PID=$!
_wait_iters=0
until curl -sf "http://localhost:${API_PORT}/healthz" >/dev/null 2>&1; do
  sleep 0.5
  _wait_iters=$((_wait_iters + 1))
  if [ "$_wait_iters" -ge 120 ]; then
    echo "FATAL: API never became ready on :${API_PORT} after 60s"
    exit 1
  fi
done

echo "==> Checking port ${WEB_PORT} is free"
check_port_free "$WEB_PORT"

echo "==> Starting the lite dev server on :${WEB_PORT}"
( cd "$REPO" && VITE_E2E_API_PORT="$API_PORT" pnpm --filter @mind-imprint/lite-web dev --port "$WEB_PORT" --strictPort ) &
WEB_PID=$!
_wait_iters=0
until curl -sf "http://localhost:${WEB_PORT}" >/dev/null 2>&1; do
  sleep 0.5
  _wait_iters=$((_wait_iters + 1))
  if [ "$_wait_iters" -ge 120 ]; then
    echo "FATAL: lite dev server never became ready on :${WEB_PORT} after 60s"
    exit 1
  fi
done

echo "==> Running Playwright"
( cd "$REPO/apps/lite-web" && E2E_PG_CONTAINER="$PG_CONTAINER" E2E_BASE_URL="http://localhost:${WEB_PORT}" pnpm e2e "$@" )
