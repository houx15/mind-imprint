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

# How much of / has to be free before we start building. One api build writes
# roughly 5 GB of layers and cache, and running out MID-BUILD is what wedged
# 2026-09-10: the image finished, then `migrate` died on "no space left on
# device" with the box at 100%.
MIN_FREE_GB=12
# Build cache older than this is dropped on every deploy. Recent cache is what
# makes a same-day rebuild fast; a week-old layer is just rent.
CACHE_KEEP=48h
# Pre-migration pg_dumps to keep. They are snapshots taken seconds before each
# migration, not the backup system of record, and they had never been deleted:
# 188 files / 2.2 GB going back to 2026-08-05 by the time anyone looked.
BACKUPS_KEEP=30

avail_gb() { df -P -k / | awk 'NR==2 {print int($4/1024/1024)}'; }

# reclaim_disk — get the box back under its own control BEFORE a build.
#
# 🚨 `docker builder prune` WITHOUT an `until` filter reclaims NOTHING here,
# whatever else you pass it. The old line was
#
#     docker builder prune -f --keep-storage 5GB
#
# which reads like a 5 GB cap and has never once deleted a byte: measured
# 2026-09-10 against 9.8 GB of cache, it reported `Total: 0B`. Adding `-a` does
# not help either, and neither does the modern spelling `--reserved-space` —
# BuildKit counts these records as in use because they are shared with images
# that still exist, and only an explicit age filter overrides that. The same
# box, same cache, `--filter until=72h` freed 10.5 GB. So the age filter is not
# a nicety here; it is the only lever that works.
#
# NEVER `image prune -a` / `system prune -a`: on this China host that also drops
# the tagged base images (golang / node / nginx / gcr.io-distroless) which
# cannot be re-pulled directly, and the next build has nothing to build on.
# `image prune -f` (dangling only) and `builder prune` cannot touch them.
#
# Takes the phase: `pre` (before building) or `post` (after). Only `pre` will
# fall back to dropping cache it cannot age out — after a build, that cache IS
# this build's, and throwing it away buys disk we do not need yet at the price
# of making every single build a cold one. If `post` leaves the box tight, the
# next deploy's `pre` pass clears it with a day's distance.
reclaim_disk() {
  local phase="$1"
  step "reclaim disk ($phase; free $(avail_gb)GB now, want ${MIN_FREE_GB}GB to build)"
  docker image prune -f || true
  docker builder prune -af --filter "until=$CACHE_KEEP" || true

  # Pre-migration dumps, newest kept. `ls -t` orders by mtime, so this keeps the
  # most recent N whatever they are named.
  if [ -d backups ]; then
    local old
    old=$(ls -1t backups/backup-*.sql 2>/dev/null | tail -n +$((BACKUPS_KEEP + 1)))
    if [ -n "$old" ]; then
      echo "$old" | xargs -r rm -f
      echo "dropped $(echo "$old" | wc -l) old pg_dump(s), kept the newest $BACKUPS_KEEP"
    fi
  fi

  # Still tight going INTO a build? Take the rest of the cache too. A slow build
  # beats one that cannot write, and the only thing lost is rebuild speed.
  if [ "$phase" = pre ] && [ "$(avail_gb)" -lt "$MIN_FREE_GB" ]; then
    echo "still under ${MIN_FREE_GB}GB — dropping all build cache except the last hour"
    docker builder prune -af --filter until=1h || true
  fi
  echo "free after reclaim: $(avail_gb)GB"
}

cd "$REPO" || fail "no repo at $REPO"

step "sync source to $REF"
git fetch origin || fail "git fetch"
git reset --hard "$REF" || fail "git reset"      # leaves git-ignored deploy/.env.prod untouched
git --no-pager log --oneline -1

# Reclaim BEFORE building, not only after. The post-build prune that used to be
# the only one cannot help a build that has already run the disk to zero — and
# a half-finished deploy is the worst state to discover it from.
reclaim_disk pre
if [ "$(avail_gb)" -lt "$MIN_FREE_GB" ]; then
  df -h /
  fail "only $(avail_gb)GB free after reclaiming, need ${MIN_FREE_GB}GB — something other than build cache and old backups is filling /, look before deleting"
fi

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

# And again afterwards: `compose build` retags mindimprint-{api,web,lite-web}
# :latest onto the NEW image, leaving the previous build's layers DANGLING, and
# this build just added its own cache. Same function, so the two passes cannot
# drift apart.
reclaim_disk post
df -h / | tail -1

step "container status (check CREATED age reflects this deploy)"
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.CreatedAt}}'
echo
echo "DEPLOY OK ($MODE @ $(git rev-parse --short HEAD))"
