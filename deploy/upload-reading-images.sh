#!/usr/bin/env bash
# Upload the 分级阅读库 photographs to the STUDENT platform OSS bucket.
#
# Source art is produced by deploy/reading-library/make_images.py, which resizes
# the press originals (177 MB) down to 1600px WebP (7 MB) and writes a manifest
# naming each object key:
#
#     deploy/reading-library/dist/images/asian-games-2.webp  ->  web/reading/v1/asian-games-2.webp
#
# Keys are DETERMINISTIC and are baked into apps/api/internal/library/articles.json
# by build.py, so nothing here writes a manifest back — the key is a pure
# function of the source filename. Re-encoding the library later should ship to
# a new `v1` -> `v2` prefix rather than overwriting: objects carry a 30-day
# Cache-Control, and reusing a key would serve the old picture for a month.
#
# The student bucket is private with URL 鉴权. Reading URLs are minted per
# request by the API (oss.Service.SignDownload, called from
# internal/api/library.go), never as plain CDN links, so this script only
# writes — it does not verify a public read.
#
# This file contains NO secrets and is committed. Credentials are read at run
# time from the gitignored deploy env file (.deploy-local/env.prod), the same
# OSS_* key pair the API uses in production.
#
# Usage:
#   deploy/upload-reading-images.sh            # upload all 60 pictures
#   deploy/upload-reading-images.sh --dry-run  # list what would be uploaded
set -uo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
ENV_FILE="${READING_OSS_ENV:-$ROOT/.deploy-local/env.prod}"
SRC_DIR="${READING_IMAGE_DIR:-$HERE/reading-library/dist/images}"
KEY_PREFIX="web/reading/v1"
# A month. These pictures never change in place — a new encode gets a new
# prefix — so there is nothing to invalidate.
CACHE_CONTROL="public, max-age=2592000"

fail() { echo "UPLOAD FAILED: $*" >&2; exit 1; }

DRY_RUN=0
[ "${1:-}" = "--dry-run" ] && DRY_RUN=1

[ -f "$ENV_FILE" ] || fail "no credentials at $ENV_FILE"
[ -d "$SRC_DIR" ]  || fail "no processed images at $SRC_DIR — run deploy/reading-library/make_images.py first"

# shellcheck disable=SC1090
set -a; . "$ENV_FILE"; set +a

: "${OSS_BUCKET:?OSS_BUCKET missing from $ENV_FILE}"
: "${OSS_ENDPOINT:?OSS_ENDPOINT missing from $ENV_FILE}"
: "${OSS_ACCESS_KEY_ID:?OSS_ACCESS_KEY_ID missing from $ENV_FILE}"
: "${OSS_ACCESS_KEY_SECRET:?OSS_ACCESS_KEY_SECRET missing from $ENV_FILE}"

# Accept the endpoint whether or not the bucket is already prefixed.
HOST="$OSS_ENDPOINT"
HOST="${HOST#https://}"; HOST="${HOST#http://}"
case "$HOST" in
  "$OSS_BUCKET".*) ;;
  *) HOST="$OSS_BUCKET.$HOST" ;;
esac

BODY="$ROOT/.oss_put_body"

# OSS signature V1: VERB\nContent-MD5\nContent-Type\nDate\nCanonicalizedResource
put_object() {
  local file="$1" key="$2"
  local ctype date sig canon code
  ctype="image/webp"
  date="$(LC_ALL=C TZ=GMT date '+%a, %d %b %Y %H:%M:%S GMT')"
  canon="/${OSS_BUCKET}/${key}"
  sig="$(printf '%b' "PUT\n\n${ctype}\n${date}\n${canon}" \
        | openssl dgst -sha1 -hmac "$OSS_ACCESS_KEY_SECRET" -binary \
        | openssl base64)"
  code=$(curl -sS -o "$BODY" -w '%{http_code}' -X PUT \
    "https://${HOST}/${key}" \
    -H "Host: ${HOST}" \
    -H "Date: ${date}" \
    -H "Content-Type: ${ctype}" \
    -H "Cache-Control: ${CACHE_CONTROL}" \
    -H "Authorization: OSS ${OSS_ACCESS_KEY_ID}:${sig}" \
    --data-binary @"$file")
  if [ "$code" = "200" ]; then
    printf '  %-56s %8s bytes\n' "$key" "$(wc -c < "$file" | tr -d ' ')"
    return 0
  fi
  echo "  PUT $key -> HTTP $code" >&2
  sed -e 's/^/      /' "$BODY" >&2 || true
  return 1
}

echo "=== upload reading-library pictures -> oss://$OSS_BUCKET/$KEY_PREFIX/ ==="
count=0; failed=0
for file in "$SRC_DIR"/*.webp; do
  [ -e "$file" ] || fail "no .webp files in $SRC_DIR"
  key="$KEY_PREFIX/$(basename "$file")"
  if [ "$DRY_RUN" = 1 ]; then
    printf '  would PUT %-56s %8s bytes\n' "$key" "$(wc -c < "$file" | tr -d ' ')"
    count=$((count + 1))
    continue
  fi
  if put_object "$file" "$key"; then count=$((count + 1)); else failed=$((failed + 1)); fi
done
rm -f "$BODY"

echo "--- $count uploaded, $failed failed ---"
[ "$failed" = 0 ] || exit 1
