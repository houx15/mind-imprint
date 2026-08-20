#!/usr/bin/env bash
# Upload the v2 tool-card cover art to the STUDENT platform OSS bucket.
#
# Source art lives at docs/reference/card_webp_v2/<中文名>-<n>.webp, four colorway
# variants per card (1 white / 2 black / 3 green / 4 blue). This script renames
# each to its stable card id and PUTs it to a DETERMINISTIC object key:
#
#     docs/reference/card_webp_v2/信源辨识-1.webp  ->  web/cards/v2/craap-1.webp
#
# The backend resolves covers by that key at request time (cards.CoverKey), so no
# manifest is needed — the key is a pure function of (cardId, theme). The student
# bucket is private with URL 鉴权; covers are served through short-lived signed
# URLs minted by the /cards/catalog endpoint, never as plain CDN links, so this
# script only writes — it does not verify a public read.
#
# This file contains NO secrets and is committed. Credentials are read at run
# time from the gitignored deploy env file (.deploy-local/env.prod), the same
# OSS_* key pair the API uses in production.
#
# Usage:
#   deploy/upload-card-covers.sh            # upload all 33 cards × 4 variants
#   deploy/upload-card-covers.sh --dry-run  # list what would be uploaded, no PUT
set -uo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
ENV_FILE="${CARD_OSS_ENV:-$ROOT/.deploy-local/env.prod}"
SRC_DIR="${CARD_ART_DIR:-$ROOT/docs/reference/card_webp_v2}"
KEY_PREFIX="web/cards/v2"

fail() { echo "UPLOAD FAILED: $*" >&2; exit 1; }

DRY_RUN=0
[ "${1:-}" = "--dry-run" ] && DRY_RUN=1

[ -f "$ENV_FILE" ] || fail "no credentials at $ENV_FILE"
[ -d "$SRC_DIR" ]  || fail "no source art at $SRC_DIR"

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

# 中文名 -> card id. One line per card (33; toulmin has no v2 art and is skipped
# deliberately). Order is irrelevant. Keep in sync with the card registry
# (packages/contracts/cards/*.json) and cards.CoverKey's coverless set.
MAP="
AI使用决策树=ai-decision-tree
AI边界和幻觉=ai-boundary
OPCVL=opcvl
PEE=pee
价值判断=fact-opinion-value
传播学=lens-communication
伦理判断=ethics-lenses
伦理学=lens-ethics
信源辨识=craap
元认知=metacognition
兔子洞=rabbit-hole
历史学=lens-history
多模态解构=multimodal-decode
学习报告=learning-report
情感对齐=emotional-alignment
提问卡=question-card
数据与统计=data-literacy
检索方向审视=search-plan
横向核查=sift
法学=lens-law
社会学=lens-society
科学方法论=lens-methods
立场光谱=belief-spectrum
系统科学=lens-systems
经济学=lens-economics
营销与否认套路=spin-detector
视角对照矩阵=perspective-matrix
认知者视角=knower-perspective
让步段=concession
论证地图=argument-map
话语分析=cda
资金链溯源=money-trail
逻辑学=lens-logic
"

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
  code=$(curl -sS -o "$ROOT/.oss_put_body" -w '%{http_code}' -X PUT \
    "https://${HOST}/${key}" \
    -H "Host: ${HOST}" \
    -H "Date: ${date}" \
    -H "Content-Type: ${ctype}" \
    -H "Cache-Control: public, max-age=86400" \
    -H "Authorization: OSS ${OSS_ACCESS_KEY_ID}:${sig}" \
    --data-binary @"$file")
  if [ "$code" = "200" ]; then
    printf '  %-34s %8s bytes\n' "$key" "$(wc -c < "$file" | tr -d ' ')"
    return 0
  fi
  echo "  PUT $key -> HTTP $code" >&2
  sed -e 's/^/      /' "$ROOT/.oss_put_body" >&2 2>&1 || true
  return 1
}

echo "=== upload v2 card covers -> oss://$OSS_BUCKET/$KEY_PREFIX/ ==="
count=0; failed=0; missing=0
while IFS='=' read -r zh id; do
  [ -z "$zh" ] && continue
  for n in 1 2 3 4; do
    file="$SRC_DIR/${zh}-${n}.webp"
    key="${KEY_PREFIX}/${id}-${n}.webp"
    if [ ! -f "$file" ]; then
      echo "  MISSING source: $file" >&2
      missing=$((missing + 1))
      continue
    fi
    if [ "$DRY_RUN" = "1" ]; then
      printf '  %-34s <- %s\n' "$key" "${zh}-${n}.webp"
      count=$((count + 1))
      continue
    fi
    put_object "$file" "$key" || failed=$((failed + 1))
    count=$((count + 1))
  done
done <<< "$MAP"

rm -f "$ROOT/.oss_put_body"
echo
echo "processed $count · failed $failed · missing $missing"
[ "$missing" -eq 0 ] || fail "$missing source file(s) not found — check the 中文->id map against docs/reference/card_webp_v2/"
[ "$failed" -eq 0 ]  || fail "$failed object(s) failed to upload"
[ "$DRY_RUN" = "1" ] && { echo "DRY RUN — nothing uploaded"; exit 0; }
echo "CARD COVER UPLOAD OK — $count objects"
