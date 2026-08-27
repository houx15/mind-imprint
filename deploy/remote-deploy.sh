#!/usr/bin/env bash
# Standardized production deploy — runs ON the ECS, from the repo root.
#
# Do not run this by hand over a raw ssh string; drive it from the local
# wrapper `.deploy-local/deploy.sh <web|api|full>` which pipes this file to the
# server over the deploy key. Committing the script (no secrets here) means the
# exact flags — especially `--env-file deploy/.env.prod`, without which the web
# bundle builds with an EMPTY VITE_API_BASE_URL and login 405s — can never be
# forgotten again.
#
# Usage (on server):  bash -s -- <web|api|lite|full> [git-ref]
#   web   rebuild + restart the web container only (fast; no DB touch)
#   lite  rebuild + restart the lite-web container only (fast; no DB touch)
#   api   backup DB → migrate+seed → rebuild + restart the api container
#   full  api steps, then web, then lite
# git-ref defaults to origin/main.
#
# `full` deploys BOTH frontends. They are built from one source tree and talk
# to one API, so shipping only half of a change that touches shared components
# is how the two editions drift apart.
set -uo pipefail

# NOTE: this script is delivered to the server via `ssh 'bash -s' < this-file`,
# so bash reads it from stdin. Every `docker compose exec/run` therefore MUST
# carry `< /dev/null` — otherwise it consumes the rest of the piped script from
# stdin and silently truncates the remaining deploy steps. Do not remove them.

MODE="${1:-}"
REF="${2:-origin/main}"
case "$MODE" in web|api|lite|full) ;; *) echo "usage: bash -s -- <web|api|lite|full> [git-ref]"; exit 2;; esac

REPO=~/mind-imprint
COMPOSE=(docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml)
API_LOCAL="http://127.0.0.1:8090"
WEB_PUBLIC="https://mind-web.uni-robot.cn"
API_HOST="mind-api.uni-robot.cn"   # what the web bundle MUST embed

fail() { echo "DEPLOY FAILED: $*" >&2; exit 1; }
step() { echo; echo "=== $* ==="; }

cd "$REPO" || fail "no repo at $REPO"

step "sync source to $REF"
git fetch origin || fail "git fetch"
git reset --hard "$REF" || fail "git reset"      # leaves git-ignored deploy/.env.prod untouched
git --no-pager log --oneline -1

deploy_api() {
  step "backup DB (pg_dump) before migrating"
  mkdir -p backups
  local out="backups/backup-$(date +%F-%H%M%S).sql"
  if "${COMPOSE[@]}" up -d db && "${COMPOSE[@]}" exec -T db pg_dump -U mindimprint mindimprint > "$out" < /dev/null; then
    echo "backup -> $out ($(wc -c < "$out") bytes)"
    [ -s "$out" ] || fail "backup is empty — aborting before migrate"
  else
    fail "pg_dump backup failed — aborting before migrate"
  fi

  step "migrate + seed (goose, idempotent)"
  "${COMPOSE[@]}" run --rm --build api -migrate-up < /dev/null || fail "migrate"

  step "rebuild + restart api"
  "${COMPOSE[@]}" up -d --build api || fail "api up"

  step "verify api health"
  # Poll each endpoint — a fresh container needs a moment to bind its port, so a
  # single immediate curl races the boot and returns 000 (false negative). Retry
  # up to ~20s before giving up.
  for ep in healthz readyz; do
    code=000
    for _ in $(seq 1 10); do
      code=$(curl -s -o /dev/null -w '%{http_code}' "$API_LOCAL/$ep")
      [ "$code" = "200" ] && break
      sleep 2
    done
    echo "$ep -> $code"
    [ "$code" = "200" ] || fail "$ep returned $code (expected 200 after retries)"
  done
}

deploy_web() {
  step "rebuild + restart web (WITH --env-file so VITE_API_BASE_URL is baked in)"
  "${COMPOSE[@]}" up -d --build web || fail "web up"

  step "verify built bundle embeds the absolute API host (in-container, deterministic)"
  # Grep the ACTUAL built artifact inside the container — not over the network.
  # A network fetch right after restart races nginx and can truncate the 1MB
  # bundle mid-stream, missing the host string and failing a healthy deploy.
  # The container filesystem is authoritative and immune to that race.
  local hits
  hits=$("${COMPOSE[@]}" exec -T web sh -c "grep -rl '$API_HOST' /usr/share/nginx/html/assets/ 2>/dev/null | head -3" < /dev/null)
  if [ -n "$hits" ]; then
    echo "OK bundle embeds $API_HOST (login will POST directly, no 405):"
    echo "$hits" | sed 's/^/    /'
  else
    fail "built bundle does NOT embed $API_HOST — VITE_API_BASE_URL was empty (login would 405). Check deploy/.env.prod + --env-file."
  fi
}

deploy_lite() {
  step "rebuild + restart lite-web (WITH --env-file so VITE_API_BASE_URL is baked in)"
  "${COMPOSE[@]}" up -d --build lite-web || fail "lite-web up"

  step "verify built lite bundle embeds the absolute API host (in-container, deterministic)"
  # Same reasoning as deploy_web: read the artifact from the container, not
  # over the network, so a restart race cannot fail a healthy deploy.
  local hits
  hits=$("${COMPOSE[@]}" exec -T lite-web sh -c "grep -rl '$API_HOST' /usr/share/nginx/html/assets/ 2>/dev/null | head -3" < /dev/null)
  if [ -n "$hits" ]; then
    echo "OK lite bundle embeds $API_HOST:"
    echo "$hits" | sed 's/^/    /'
  else
    fail "built lite bundle does NOT embed $API_HOST — VITE_API_BASE_URL was empty. Check deploy/.env.prod + --env-file."
  fi
}

case "$MODE" in
  api)  deploy_api ;;
  web)  deploy_web ;;
  lite) deploy_lite ;;
  full) deploy_api; deploy_web; deploy_lite ;;
esac

# Reclaim disk from the build we just superseded: `compose build` retags
# mindimprint-{api,web}:latest onto the NEW image, leaving the previous build's
# layers DANGLING (untagged). Prune only those + old build cache — this keeps the
# 40G host from filling across many deploys. NEVER `image prune -a`: on this
# China host it would also drop the tagged base images (golang / nginx /
# gcr.io-distroless) that can't be re-pulled directly, breaking the next build.
step "prune superseded (dangling) images + old build cache"
docker image prune -f || true
docker builder prune -f --keep-storage 5GB || true

step "container status (check CREATED age reflects this deploy)"
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.CreatedAt}}'
echo
echo "DEPLOY OK ($MODE @ $(git rev-parse --short HEAD))"
