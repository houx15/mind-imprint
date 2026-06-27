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

cleanup() {
  set +e
  [ -n "$WEB_PID" ] && kill "$WEB_PID" 2>/dev/null
  [ -n "$API_PID" ] && kill "$API_PID" 2>/dev/null
  docker rm -f "$PG_CONTAINER" >/dev/null 2>&1
}
trap cleanup EXIT INT TERM

command -v docker >/dev/null 2>&1 || { echo "FATAL: Docker is required for the throwaway Postgres."; exit 1; }
[ -f "$REPO/apps/api/.env.local" ] || { echo "FATAL: create apps/api/.env.local with DEEPSEEK_API_KEY (see RUNBOOK.md)."; exit 1; }
grep -qE '^DEEPSEEK_API_KEY=.+' "$REPO/apps/api/.env.local" || echo "WARN: DEEPSEEK_API_KEY looks empty — live-model steps will fail."

echo "==> Booting throwaway Postgres on :${PG_PORT}"
docker rm -f "$PG_CONTAINER" >/dev/null 2>&1 || true
docker run -d --name "$PG_CONTAINER" -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=mindimprint -p "${PG_PORT}:5432" postgres:16 >/dev/null
until docker exec "$PG_CONTAINER" pg_isready -U postgres >/dev/null 2>&1; do sleep 0.5; done

echo "==> Applying migrations + seed"
( cd "$REPO/apps/api" && DATABASE_URL="$DB_URL" go run ./cmd/api -migrate-up )

echo "==> Starting API on :8080"
# DATABASE_URL/COOKIE_SECURE/CORS_ORIGINS are exported here; godotenv does NOT
# override already-set env vars, so .env.local still supplies DEEPSEEK_API_KEY.
( cd "$REPO/apps/api" && DATABASE_URL="$DB_URL" COOKIE_SECURE=false CORS_ORIGINS=http://localhost:5173 go run ./cmd/api ) &
API_PID=$!
until curl -sf http://localhost:8080/healthz >/dev/null; do sleep 0.5; done

echo "==> Starting web dev server on :5173"
( cd "$REPO" && pnpm --filter web dev --port 5173 --strictPort ) &
WEB_PID=$!
until curl -sf http://localhost:5173 >/dev/null; do sleep 0.5; done

echo "==> Running Playwright"
( cd "$REPO/apps/web" && pnpm e2e "$@" )
