#!/usr/bin/env bash
# Verify one uploaded library picture reads back through the CDN.
#
# The student bucket is private with URL 鉴权, so a plain CDN GET is expected to
# be refused; the only correct read is a signed one. This asks the production
# API to sign a key (the admin-key route, same one backend scripts use) and
# then fetches it, printing the status and the bytes that came back.
#
# Usage: READING_OSS_ENV=/path/to/env.prod deploy/reading-library/probe_cdn.sh [objectKey]
set -uo pipefail

ENV_FILE="${READING_OSS_ENV:?set READING_OSS_ENV to the deploy env file}"
API="${MIND_API:-https://mind-api.uni-robot.cn}"
KEY="${1:-web/reading/v1/asian-games-2.webp}"

# shellcheck disable=SC1090
set -a; . "$ENV_FILE"; set +a
: "${OSS_ADMIN_KEY:?OSS_ADMIN_KEY missing from $ENV_FILE}"

RESP="$(curl -sS -X POST "$API/api/v1/oss/resolve-url" \
  -H "Authorization: Bearer $OSS_ADMIN_KEY" \
  -H 'Content-Type: application/json' \
  -d "{\"objectKey\":\"$KEY\"}")"
echo "resolve-url -> $RESP" | cut -c1-200

URL="$(printf '%s' "$RESP" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("url",""))')"
[ -n "$URL" ] || { echo "no signed url came back" >&2; exit 1; }

OUT="${TMPDIR:-/tmp}/mind-reading-probe.webp"
curl -sS -o "$OUT" -w 'signed GET  HTTP %{http_code}  %{size_download} bytes  %{content_type}\n' "$URL"
file "$OUT"

# And the same key WITHOUT a signature, which must be refused.
BASE="${URL%%\?*}"
curl -sS -o /dev/null -w 'unsigned GET  HTTP %{http_code}  (403 expected)\n' "$BASE"
