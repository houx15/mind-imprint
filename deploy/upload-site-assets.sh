#!/usr/bin/env bash
# Upload MARKETING SITE media to its own Aliyun OSS bucket.
#
# Scope, deliberately narrow: this touches ONLY the site's bucket. The student
# platform's object storage is a different bucket, a different CDN host and a
# different key pair, reached through the API's /oss/* admin route — none of
# which this script knows about. Don't merge the two.
#
# This file contains NO secrets and is committed. Credentials are read at run
# time from a gitignored env file:
#
#   cp deploy/site-oss.env.example .deploy-local/site-oss.env   # then fill it in
#
# Usage:
#   deploy/upload-site-assets.sh                     # sync apps/site/assets/ -> bucket root
#   deploy/upload-site-assets.sh path/to/file.png home/hero.png   # one file to one key
#   deploy/upload-site-assets.sh --check             # prove the CDN can read the bucket
#
# Why no credential ends up on the server: the bucket is private, but the CDN is
# authorised to read it, so visitors fetch plain URLs. Only uploading needs a
# key, and uploading happens here, from a developer machine.
set -uo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
ENV_FILE="${SITE_OSS_ENV:-$ROOT/.deploy-local/site-oss.env}"
SRC_DIR="${SITE_ASSET_DIR:-$ROOT/apps/site/assets}"
HEALTH_KEY="_meta/healthcheck.txt"

fail() { echo "UPLOAD FAILED: $*" >&2; exit 1; }

[ -f "$ENV_FILE" ] || fail "no credentials at $ENV_FILE
  cp deploy/site-oss.env.example .deploy-local/site-oss.env and fill it in."

# shellcheck disable=SC1090
set -a; . "$ENV_FILE"; set +a

: "${OSS_BUCKET:?OSS_BUCKET missing from $ENV_FILE}"
: "${OSS_ENDPOINT:?OSS_ENDPOINT missing from $ENV_FILE}"
: "${OSS_ACCESS_KEY_ID:?OSS_ACCESS_KEY_ID missing from $ENV_FILE}"
: "${OSS_ACCESS_KEY_SECRET:?OSS_ACCESS_KEY_SECRET missing from $ENV_FILE}"
: "${ASSET_BASE_URL:?ASSET_BASE_URL missing from $ENV_FILE}"

# Aliyun's console shows the endpoint with the bucket already prefixed
# (mind-open.oss-cn-beijing.aliyuncs.com). Accept it either way.
HOST="$OSS_ENDPOINT"
case "$HOST" in
  "$OSS_BUCKET".*) ;;
  *) HOST="$OSS_BUCKET.$HOST" ;;
esac

CACHE_CONTROL="${ASSET_CACHE_CONTROL:-public, max-age=86400}"

content_type_for() {
  case "${1##*.}" in
    png)          echo "image/png" ;;
    jpg|jpeg)     echo "image/jpeg" ;;
    webp)         echo "image/webp" ;;
    avif)         echo "image/avif" ;;
    gif)          echo "image/gif" ;;
    svg)          echo "image/svg+xml" ;;
    mp4)          echo "video/mp4" ;;
    webm)         echo "video/webm" ;;
    pdf)          echo "application/pdf" ;;
    vtt)          echo "text/vtt" ;;
    txt)          echo "text/plain" ;;
    json)         echo "application/json" ;;
    *)            echo "application/octet-stream" ;;
  esac
}

# OSS signature V1:
#   VERB \n Content-MD5 \n Content-Type \n Date \n CanonicalizedResource
# (no x-oss-* headers are sent, so CanonicalizedOSSHeaders is empty)
put_object() {
  local file="$1" key="$2"
  local ctype date sig canon code
  ctype="$(content_type_for "$file")"
  date="$(LC_ALL=C TZ=GMT date '+%a, %d %b %Y %H:%M:%S GMT')"
  canon="/${OSS_BUCKET}/${key}"
  sig="$(printf '%b' "PUT\n\n${ctype}\n${date}\n${canon}" \
        | openssl dgst -sha1 -hmac "$OSS_ACCESS_KEY_SECRET" -binary \
        | openssl base64)"

  code=$(curl -sS -o /tmp/.oss_put_body -w '%{http_code}' -X PUT \
    "https://${HOST}/${key}" \
    -H "Host: ${HOST}" \
    -H "Date: ${date}" \
    -H "Content-Type: ${ctype}" \
    -H "Cache-Control: ${CACHE_CONTROL}" \
    -H "Authorization: OSS ${OSS_ACCESS_KEY_ID}:${sig}" \
    --data-binary @"$file")

  if [ "$code" = "200" ]; then
    printf '  %-46s %8s bytes  %s\n' "$key" "$(wc -c < "$file" | tr -d ' ')" "$ctype"
    return 0
  fi
  echo "  PUT $key -> HTTP $code" >&2
  sed -e 's/^/      /' /tmp/.oss_put_body >&2 2>/dev/null || true
  return 1
}

verify_cdn() {
  local key="$1" code
  code=$(curl -sS -o /dev/null -w '%{http_code}' "${ASSET_BASE_URL%/}/${key}")
  echo "  ${ASSET_BASE_URL%/}/${key} -> $code"
  [ "$code" = "200" ]
}

# --- --check : prove the whole path works end to end ------------------------
if [ "${1:-}" = "--check" ]; then
  echo "=== upload a probe object ==="
  tmp=$(mktemp); echo "mind-imprint site assets ok" > "$tmp"
  # name it .txt so the content type is right
  probe="$tmp.txt"; mv "$tmp" "$probe"
  put_object "$probe" "$HEALTH_KEY" || fail "cannot write to bucket $OSS_BUCKET — check the key pair"
  rm -f "$probe"

  echo
  echo "=== read it back through the CDN (proves CDN->private bucket access) ==="
  # A brand-new object can take a moment to be fetchable at the edge.
  for _ in $(seq 1 5); do verify_cdn "$HEALTH_KEY" && ok=1 && break; sleep 3; done
  [ "${ok:-}" = "1" ] || fail "CDN cannot serve ${HEALTH_KEY}.
  The object uploaded fine, so the bucket and the key pair are OK — this is a
  CDN config issue. Assets are always served through the CDN, never straight
  from OSS, so check on the Aliyun console:
    · is ${ASSET_BASE_URL#https://} a CDN accelerated domain whose origin is
      the OSS domain ${OSS_BUCKET}.oss-cn-<region>.aliyuncs.com?
      (if DNS still CNAMEs to *.taihangpkx.cn it is bound to the OSS custom
       domain, not to CDN — that path cannot read a private bucket)
    · is 私有 Bucket 回源 authorised, so the CDN may read the private bucket?
    · does the domain have an HTTPS certificate that matches it?
    · is URL 鉴权 (URL auth) switched OFF? It must be — the marketing site is
      public and a static page cannot sign a URL at request time."
  echo
  echo "ASSET PIPELINE OK — ${ASSET_BASE_URL} serves ${OSS_BUCKET}"
  exit 0
fi

# --- single file: <localFile> <key> -----------------------------------------
if [ $# -eq 2 ]; then
  [ -f "$1" ] || fail "no such file: $1"
  echo "=== upload 1 file to $OSS_BUCKET ==="
  put_object "$1" "$2" || fail "upload"
  echo
  echo "=== verify via CDN ==="
  verify_cdn "$2" || fail "uploaded, but the CDN will not serve it (see --check)"
  exit 0
fi

[ $# -eq 0 ] || fail "usage: $0 [--check] | [<localFile> <key>]"

# --- default: sync the whole asset dir --------------------------------------
if [ ! -d "$SRC_DIR" ]; then
  echo "no asset directory at $SRC_DIR — nothing to upload."
  echo "Create it and drop files in; the tree maps 1:1 onto keys, e.g."
  echo "  apps/site/assets/home/course-player.png  ->  home/course-player.png"
  echo "  used in a page as:  <Figure asset=\"home/course-player.png\" … />"
  exit 0
fi

echo "=== sync $SRC_DIR -> oss://$OSS_BUCKET/ ==="
count=0; failed=0
while IFS= read -r file; do
  key="${file#"$SRC_DIR"/}"
  put_object "$file" "$key" || failed=$((failed + 1))
  count=$((count + 1))
done < <(find "$SRC_DIR" -type f ! -name '.DS_Store' | sort)

echo
echo "uploaded $((count - failed))/$count"
[ "$failed" -eq 0 ] || fail "$failed object(s) failed"

echo
echo "=== spot-check the newest object via CDN ==="
newest=$(find "$SRC_DIR" -type f ! -name '.DS_Store' | sort | tail -1)
[ -n "$newest" ] && verify_cdn "${newest#"$SRC_DIR"/}"

echo
echo "ASSET SYNC OK"
