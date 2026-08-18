#!/usr/bin/env bash
# Standardized production deploy for the MARKETING SITE — runs ON the ECS.
#
# Separate from deploy/remote-deploy.sh on purpose. That script drives the
# student platform (api + db + SPA) out of ~/mind-imprint; this one drives the
# marketing site out of its OWN checkout at ~/mind-imprint-site, with its own
# compose project. The two never touch:
#
#   ~/mind-imprint       mindimprint       api :8090 · web :8091 · db (internal)
#   ~/mind-imprint-site  mindimprint-site  site :8092
#
# A separate checkout is the point. If both stacks shared one working tree,
# deploying the site at a feature ref would leave the product's tree parked on
# that ref, and the next product build would pick it up. They don't share one.
#
# Do not run this by hand over a raw ssh string; drive it from the local wrapper
# `.deploy-local/deploy-site.sh [git-ref]`, which pipes this file to the server
# over the deploy key.
#
# Usage (on server):  bash -s -- [git-ref]
# git-ref defaults to origin/main.
set -uo pipefail

# NOTE: this script is delivered via `ssh 'bash -s' < this-file`, so bash reads
# it from stdin. Every `docker compose exec/run` therefore MUST carry
# `< /dev/null` — otherwise it eats the rest of the piped script from stdin and
# silently truncates the remaining steps. Do not remove them.

REF="${1:-origin/main}"

REPO=~/mind-imprint-site
ORIGIN=git@github.com:houx15/mind-imprint.git
COMPOSE=(docker compose -f deploy/docker-compose.site.yml)
SITE_LOCAL="http://127.0.0.1:8092"
APP_HOST="mind-web.uni-robot.cn"   # what the site's CTAs MUST point at

fail() { echo "SITE DEPLOY FAILED: $*" >&2; exit 1; }
step() { echo; echo "=== $* ==="; }

step "ensure the site has its own checkout at $REPO"
if [ ! -d "$REPO/.git" ]; then
  echo "no checkout yet — cloning (one-time)"
  git clone "$ORIGIN" "$REPO" || fail "clone"
fi
cd "$REPO" || fail "no repo at $REPO"

# Guard against the two stacks ever being pointed at the same directory.
[ "$(cd "$REPO" && pwd -P)" != "$(cd ~/mind-imprint 2>/dev/null && pwd -P)" ] \
  || fail "site checkout resolves to the product checkout — refusing to deploy"

step "sync source to $REF"
git fetch origin || fail "git fetch"
git reset --hard "$REF" || fail "git reset"
git --no-pager log --oneline -1

step "rebuild + restart the site container"
"${COMPOSE[@]}" up -d --build site || fail "site up"

step "verify the built HTML embeds the app host (in-container, deterministic)"
# Grep the ACTUAL built artifact inside the container rather than over the
# network: a fetch right after restart races nginx and can truncate mid-stream,
# failing a healthy deploy. The container filesystem is authoritative.
hits=$("${COMPOSE[@]}" exec -T site sh -c "grep -rl '$APP_HOST' /usr/share/nginx/html/index.html /usr/share/nginx/html/en/index.html 2>/dev/null" < /dev/null)
if [ -n "$hits" ]; then
  echo "OK the CTAs point at $APP_HOST:"
  echo "$hits" | sed 's/^/    /'
else
  fail "built HTML does NOT reference $APP_HOST — PUBLIC_APP_URL was wrong, so every CTA would dead-end on localhost."
fi

step "verify every route actually renders"
for path in / /product /algorithm /about /en/ /en/product /en/algorithm /en/about; do
  code=000
  for _ in $(seq 1 10); do
    code=$(curl -s -o /dev/null -w '%{http_code}' "$SITE_LOCAL$path")
    [ "$code" = "200" ] && break
    sleep 2
  done
  printf '  %-18s -> %s\n' "$path" "$code"
  [ "$code" = "200" ] || fail "$path returned $code (expected 200 after retries)"
done

step "verify the retired /evaluation URL still redirects"
code=$(curl -s -o /dev/null -w '%{http_code}' "$SITE_LOCAL/evaluation")
echo "  /evaluation -> $code"
case "$code" in 200|301|302) ;; *) fail "/evaluation returned $code" ;; esac

step "verify an unknown path 404s (it must NOT fall back to the homepage)"
code=$(curl -s -o /dev/null -w '%{http_code}' "$SITE_LOCAL/definitely-not-a-page")
echo "  /definitely-not-a-page -> $code"
[ "$code" = "404" ] || fail "unknown path returned $code, expected 404 — the nginx fallback is wrong"

# Reclaim disk from the build we just superseded. NEVER `image prune -a`: on
# this China host that would also drop tagged base images (node / nginx) which
# can't be re-pulled directly, breaking the next build.
step "prune superseded (dangling) images + old build cache"
docker image prune -f || true
docker builder prune -f --keep-storage 5GB || true

step "container status"
docker ps --filter "name=mindimprint-site" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}\t{{.CreatedAt}}'
echo
echo "SITE DEPLOY OK (@ $(git rev-parse --short HEAD))"
